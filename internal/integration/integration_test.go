// Package integration — интеграционные тесты на реальном Postgres
// (docker, см. раздел «Тестовый стенд» в CLAUDE.md). Запуск:
//
//	TEST_PG_DSN=postgres://postgres:postgres@localhost:55432/kusec_test?sslmode=disable go test ./internal/integration/
//
// Без TEST_PG_DSN тесты пропускаются (обычный go test ./... их не гоняет).
package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	appDb "github.com/rendau/kusec/internal/domain/app/repo/db"
	appService "github.com/rendau/kusec/internal/domain/app/service"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	auditDb "github.com/rendau/kusec/internal/domain/audit/repo/db"
	auditService "github.com/rendau/kusec/internal/domain/audit/service"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configitemDb "github.com/rendau/kusec/internal/domain/configitem/repo/db"
	configitemService "github.com/rendau/kusec/internal/domain/configitem/service"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	configmapDb "github.com/rendau/kusec/internal/domain/configmap/repo/db"
	configmapService "github.com/rendau/kusec/internal/domain/configmap/service"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	itemDb "github.com/rendau/kusec/internal/domain/item/repo/db"
	itemService "github.com/rendau/kusec/internal/domain/item/service"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	secretDb "github.com/rendau/kusec/internal/domain/secret/repo/db"
	secretService "github.com/rendau/kusec/internal/domain/secret/service"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	usrDb "github.com/rendau/kusec/internal/domain/usr/repo/db"
	usrService "github.com/rendau/kusec/internal/domain/usr/service"
	auditRecorder "github.com/rendau/kusec/internal/service/audit"
	appUsc "github.com/rendau/kusec/internal/usecase/app"
	itemUsc "github.com/rendau/kusec/internal/usecase/item"
	"github.com/rendau/kusec/internal/util"
)

const testHashKey = "integration-hash-key"

// stack — реальный стек до usecase-слоя поверх docker Postgres.
type stack struct {
	pool *pgxpool.Pool
	txm  *mobone.TransactionManager

	sessionSvc *sessionService.Service

	appSvc        *appService.Service
	secretSvc     *secretService.Service
	itemSvc       *itemService.Service
	configMapSvc  *configmapService.Service
	configItemSvc *configitemService.Service
	auditSvc      *auditService.Service

	auditRec *auditRecorder.Service

	itemUsecase *itemUsc.Usecase
	appUsecase  *appUsc.Usecase
}

func newStack(t *testing.T) *stack {
	t.Helper()

	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN is not set; integration tests need a docker Postgres (see CLAUDE.md)")
	}

	// миграции — из каталога репозитория (путь от этого файла)
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationsPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	m, err := migrate.New("file://"+migrationsPath, dsn)
	require.NoError(t, err)
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	s := &stack{
		pool:       pool,
		txm:        mobone.NewTransactionManager(pool),
		sessionSvc: sessionService.New("integration-test"),
	}

	usrSvc := usrService.New(usrDb.New(pool))
	s.appSvc = appService.New(appDb.New(pool))
	s.secretSvc = secretService.New(secretDb.New(pool))
	s.itemSvc = itemService.New(itemDb.New(pool))
	s.configMapSvc = configmapService.New(configmapDb.New(pool))
	s.configItemSvc = configitemService.New(configitemDb.New(pool))
	s.auditSvc = auditService.New(auditDb.New(pool))

	s.auditRec = auditRecorder.New(
		s.auditSvc, usrSvc, s.appSvc, s.secretSvc, s.configMapSvc, s.sessionSvc, testHashKey,
	)

	s.itemUsecase = itemUsc.New(s.itemSvc, s.secretSvc, s.sessionSvc, s.txm, s.auditRec)
	s.appUsecase = appUsc.New(
		s.appSvc, s.secretSvc, s.itemSvc, s.configMapSvc, s.configItemSvc,
		s.sessionSvc, s.txm, s.auditRec,
	)

	return s
}

// adminCtx — сессия админа с именем (имя в сессии избавляет аудит от
// обращения к таблице usr).
func (s *stack) adminCtx() context.Context {
	return s.sessionSvc.WithContext(context.Background(), &sessionModel.Session{
		Id:     1,
		Admin:  true,
		Name:   "Integration Tester",
		Source: "api",
	})
}

// newApp создаёт app + secret напрямую через доменные сервисы (setup без аудита).
func (s *stack) newApp(t *testing.T, slug string) (appId, secretId string) {
	t.Helper()
	ctx := context.Background()

	appId, err := s.appSvc.Create(ctx, &appModel.Edit{
		Active:    new(true),
		Namespace: new("itest"),
		Name:      new("App " + slug),
		SlugName:  new(slug),
	})
	require.NoError(t, err)

	secretId, err = s.secretSvc.Create(ctx, &secretModel.Edit{
		AppId:    new(appId),
		Active:   new(true),
		SlugName: new("main"),
	})
	require.NoError(t, err)

	return appId, secretId
}

// auditRows — записи аудита сущности + сырой jsonb changes текстом.
func (s *stack) auditRows(t *testing.T, entityType, entityId string) []auditRow {
	t.Helper()

	rows, err := s.pool.Query(context.Background(), `
		select action, actor_name, coalesce(batch_id, ''), changes::text
		from audit where entity_type = $1 and entity_id = $2
		order by id
	`, entityType, entityId)
	require.NoError(t, err)
	defer rows.Close()

	result := make([]auditRow, 0)
	for rows.Next() {
		var row auditRow
		require.NoError(t, rows.Scan(&row.action, &row.actorName, &row.batchId, &row.changes))
		result = append(result, row)
	}
	require.NoError(t, rows.Err())
	return result
}

type auditRow struct {
	action    string
	actorName string
	batchId   string
	changes   string
}

func timeDaysAgo(days int) time.Time {
	return time.Now().AddDate(0, 0, -days)
}

// ── Тесты ───────────────────────────────────────────────

// Мутация и запись аудита коммитятся одной транзакцией; значение секрета не
// попадает в запись ни в каком виде — только HMAC-отпечаток и размер.
func TestItemMutationWritesAuditInSameTransaction(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "tx-audit")

	const secretValue = "Sup3r-Secret-Tx-Value-42"

	itemId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId,
		Active:   new(true),
		Key:      new("PG_PASSWORD"),
		Value:    new(secretValue),
	})
	require.NoError(t, err)

	rows := s.auditRows(t, auditModel.EntityItem, itemId)
	require.Len(t, rows, 1)
	assert.Equal(t, "create", rows[0].action)
	assert.Equal(t, "Integration Tester", rows[0].actorName)
	assert.NotContains(t, rows[0].changes, secretValue)
	assert.Contains(t, rows[0].changes, util.ValueFingerprint(testHashKey, secretValue))

	// update значения: старый и новый отпечатки, значения не утекают
	const newValue = "N3w-Secret-Value-43"
	err = s.itemUsecase.Update(ctx, itemId, &itemModel.Edit{Value: new(newValue)})
	require.NoError(t, err)

	rows = s.auditRows(t, auditModel.EntityItem, itemId)
	require.Len(t, rows, 2)
	assert.Equal(t, "update", rows[1].action)
	assert.NotContains(t, rows[1].changes, secretValue)
	assert.NotContains(t, rows[1].changes, newValue)
	assert.Contains(t, rows[1].changes, util.ValueFingerprint(testHashKey, secretValue))
	assert.Contains(t, rows[1].changes, util.ValueFingerprint(testHashKey, newValue))

	// смена только active трактуется как deactivate
	err = s.itemUsecase.Update(ctx, itemId, &itemModel.Edit{Active: new(false)})
	require.NoError(t, err)
	rows = s.auditRows(t, auditModel.EntityItem, itemId)
	require.Len(t, rows, 3)
	assert.Equal(t, "deactivate", rows[2].action)
}

// Откат транзакции отменяет и мутацию, и запись аудита: невозможно ни
// изменение без следа, ни след без изменения.
func TestTransactionRollbackRemovesBothMutationAndAudit(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "tx-rollback")

	var itemId string
	err := s.txm.TxFn(ctx, func(ctx context.Context) error {
		var err error
		itemId, err = s.itemSvc.Create(ctx, &itemModel.Edit{
			SecretId: &secretId,
			Active:   new(true),
			Key:      new("ROLLBACK_KEY"),
			Value:    new("rollback-value"),
		})
		require.NoError(t, err)

		created, _, err := s.itemSvc.Get(ctx, itemId, true)
		require.NoError(t, err)
		require.NoError(t, s.auditRec.RecordItem(ctx, nil, created, "", nil))

		// внутри транзакции обе записи видны
		_, found, err := s.itemSvc.Get(ctx, itemId, false)
		require.NoError(t, err)
		require.True(t, found)

		return fmt.Errorf("boom: force rollback")
	})
	require.Error(t, err)

	// после отката нет ни item-а, ни записи аудита
	_, found, err := s.itemSvc.Get(context.Background(), itemId, false)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, s.auditRows(t, auditModel.EntityItem, itemId))
}

// Каскадное удаление app: FK удаляет детей молча, поэтому usecase пишет
// записи аудита на каждого ребёнка с общим batch_id — пропажа ключей не
// бесследна.
func TestAppCascadeDeleteAuditsChildren(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	appId, secretId := s.newApp(t, "cascade")

	setupCtx := context.Background()
	item1, err := s.itemSvc.Create(setupCtx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("K1"), Value: new("cascade-value-1"),
	})
	require.NoError(t, err)
	item2, err := s.itemSvc.Create(setupCtx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("K2"), Value: new("cascade-value-2"),
	})
	require.NoError(t, err)

	configMapId, err := s.configMapSvc.Create(setupCtx, &configmapModel.Edit{
		AppId: &appId, Active: new(true), SlugName: new("config"),
	})
	require.NoError(t, err)
	configItem1, err := s.configItemSvc.Create(setupCtx, &configitemModel.Edit{
		ConfigMapId: &configMapId, Active: new(true), Key: new("HTTP_PORT"), Value: new("8080"),
	})
	require.NoError(t, err)

	require.NoError(t, s.appUsecase.Delete(ctx, appId))

	// записи на app и на каждого ребёнка, с общим batch_id
	appRows := s.auditRows(t, auditModel.EntityApp, appId)
	require.Len(t, appRows, 1)
	assert.Equal(t, "delete", appRows[0].action)
	batchId := appRows[0].batchId
	require.NotEmpty(t, batchId)

	for _, entity := range []struct{ entityType, id string }{
		{auditModel.EntitySecret, secretId},
		{auditModel.EntityItem, item1},
		{auditModel.EntityItem, item2},
		{auditModel.EntityConfigMap, configMapId},
		{auditModel.EntityConfigItem, configItem1},
	} {
		rows := s.auditRows(t, entity.entityType, entity.id)
		require.Len(t, rows, 1, entity.entityType)
		assert.Equal(t, "delete", rows[0].action, entity.entityType)
		assert.Equal(t, batchId, rows[0].batchId, entity.entityType)
	}

	// значения секретов не утекли и в каскадных записях
	for _, id := range []string{item1, item2} {
		rows := s.auditRows(t, auditModel.EntityItem, id)
		assert.NotContains(t, rows[0].changes, "cascade-value")
	}
	// несекретное значение config_item — как есть
	configItemRows := s.auditRows(t, auditModel.EntityConfigItem, configItem1)
	assert.Contains(t, configItemRows[0].changes, "8080")

	// сами дочерние записи удалены каскадом
	_, found, err := s.itemSvc.Get(setupCtx, item1, false)
	require.NoError(t, err)
	assert.False(t, found)
}

// Ретеншн удаляет только записи старше границы.
func TestAuditRetention(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "retention")

	itemId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("OLD_KEY"), Value: new("v"),
	})
	require.NoError(t, err)
	require.Len(t, s.auditRows(t, auditModel.EntityItem, itemId), 1)

	// состариваем запись напрямую (created_at выставляется БД)
	_, err = s.pool.Exec(context.Background(),
		`update audit set created_at = now() - interval '10 days' where entity_id = $1`, itemId)
	require.NoError(t, err)

	// граница младше записи — запись удаляется
	deleted, err := s.auditSvc.DeleteOlderThan(context.Background(), timeDaysAgo(5))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, int64(1))
	assert.Empty(t, s.auditRows(t, auditModel.EntityItem, itemId))
}

// read_only-сессия не видит значений: usecase отдаёт только размер и отпечаток.
func TestReadOnlySessionValueMasking(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "read-only")

	const secretValue = "Masked-Secret-Value-7"
	itemId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("TOKEN"), Value: new(secretValue),
	})
	require.NoError(t, err)

	roCtx := s.sessionSvc.WithContext(context.Background(), &sessionModel.Session{
		Id: 1, Admin: true, Name: "RO", Scope: "read_only", Source: "api",
	})

	masked, err := s.itemUsecase.Get(roCtx, itemId)
	require.NoError(t, err)
	assert.Empty(t, masked.Value)
	assert.Equal(t, int64(len(secretValue)), masked.ValueSize)
	assert.NotEmpty(t, masked.ValueHash)

	// полная сессия значение видит
	full, err := s.itemUsecase.Get(ctx, itemId)
	require.NoError(t, err)
	assert.Equal(t, secretValue, full.Value)
}
