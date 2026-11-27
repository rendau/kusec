package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/config"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/infra/secretbox"
	"github.com/rendau/kusec/internal/util"
)

// ── Помощники ───────────────────────────────────────────

// isolatedDSN создаёт в тестовой базе отдельную схему и возвращает DSN,
// привязанный к ней. Перевод данных на шифрование и проверка при старте
// работают со всей таблицей item: в общей схеме они задели бы строки соседних
// тестов, а результат зависел бы от порядка запуска. Схема удаляется после теста.
func isolatedDSN(t *testing.T) string {
	t.Helper()
	baseDSN := testDSN(t)
	ctx := context.Background()

	suffix := make([]byte, 6)
	_, err := rand.Read(suffix)
	require.NoError(t, err)
	schema := "itest_" + hex.EncodeToString(suffix)

	exec := func(query string) error {
		conn, err := pgx.Connect(ctx, baseDSN)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer func() { _ = conn.Close(ctx) }()
		if _, err = conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
		return nil
	}

	require.NoError(t, exec("create schema "+schema))
	t.Cleanup(func() {
		if err := exec("drop schema if exists " + schema + " cascade"); err != nil {
			t.Logf("drop schema %s: %v", schema, err)
		}
	})

	parsed, err := url.Parse(baseDSN)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// encBox — шифрование с тестовым ключом (32 байта fill); разные fill — разные ключи.
func encBox(t *testing.T, fill byte) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, secretbox.KeySize)))
	require.NoError(t, err)
	require.True(t, box.Enabled())
	return box
}

// noBox — шифрование выключено (DATA_ENCRYPTION_KEY не задан).
func noBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New("")
	require.NoError(t, err)
	require.False(t, box.Enabled())
	return box
}

// rawValue — значение item-а как оно лежит в таблице, мимо репозитория.
func (s *stack) rawValue(t *testing.T, itemId string) string {
	t.Helper()
	var value string
	err := s.pool.QueryRow(context.Background(), `select value from item where id = $1`, itemId).Scan(&value)
	require.NoError(t, err)
	return value
}

type rawItem struct {
	value     string
	updatedAt time.Time
}

// rawItems — все строки item как они лежат в таблице.
func (s *stack) rawItems(t *testing.T) map[string]rawItem {
	t.Helper()

	rows, err := s.pool.Query(context.Background(), `select id, value, updated_at from item order by id`)
	require.NoError(t, err)
	defer rows.Close()

	result := make(map[string]rawItem)
	for rows.Next() {
		var id string
		var item rawItem
		require.NoError(t, rows.Scan(&id, &item.value, &item.updatedAt))
		result[id] = item
	}
	require.NoError(t, rows.Err())
	return result
}

func (s *stack) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, s.pool.QueryRow(context.Background(), query, args...).Scan(&count))
	return count
}

// ── Тесты ───────────────────────────────────────────────

// Значение, записанное через сервис, лежит в таблице шифртекстом; чтение
// возвращает исходное значение. Всё, что выше репозитория (аудит, отпечатки),
// работает с открытым значением, как и без шифрования.
func TestItemValueIsEncryptedAtRest(t *testing.T) {
	s := newStackWith(t, isolatedDSN(t), encBox(t, 1))
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "enc-at-rest")

	const secretValue = "At-Rest-Secret-Value-1"

	edit := &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("PG_PASSWORD"), Value: new(secretValue),
	}
	itemId, err := s.itemUsecase.Create(ctx, edit)
	require.NoError(t, err)
	// доменная модель вызывающего не подменена шифртекстом
	assert.Equal(t, secretValue, *edit.Value)

	// в таблице — шифртекст
	stored := s.rawValue(t, itemId)
	assert.True(t, strings.HasPrefix(stored, secretbox.Prefix), "stored=%q", stored)
	assert.NotContains(t, stored, secretValue)
	assert.Zero(t, s.count(t, `select count(*) from item where value like '%' || $1 || '%'`, secretValue))

	// чтение — исходное значение, размер и отпечаток считаются от него же
	got, err := s.itemUsecase.Get(ctx, itemId)
	require.NoError(t, err)
	assert.Equal(t, secretValue, got.Value)
	assert.Equal(t, int64(len(secretValue)), got.ValueSize)
	assert.Equal(t, util.ValueFingerprint(config.Conf.AuditHashKey, secretValue), got.ValueHash)

	listed, _, err := s.itemUsecase.List(ctx, &itemModel.ListReq{SecretId: &secretId})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, secretValue, listed[0].Value)

	// аудит: отпечаток открытого значения, ни значения, ни шифртекста
	rows := s.auditRows(t, auditModel.EntityItem, itemId)
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].changes, util.ValueFingerprint(testHashKey, secretValue))
	assert.NotContains(t, rows[0].changes, secretValue)
	assert.NotContains(t, rows[0].changes, secretbox.Prefix)

	// обновление значения: новый шифртекст, читается новое значение
	const newValue = "At-Rest-Secret-Value-2"
	require.NoError(t, s.itemUsecase.Update(ctx, itemId, &itemModel.Edit{Value: new(newValue)}))

	storedAfterUpdate := s.rawValue(t, itemId)
	assert.True(t, strings.HasPrefix(storedAfterUpdate, secretbox.Prefix))
	assert.NotEqual(t, stored, storedAfterUpdate)
	assert.NotContains(t, storedAfterUpdate, newValue)

	got, err = s.itemUsecase.Get(ctx, itemId)
	require.NoError(t, err)
	assert.Equal(t, newValue, got.Value)

	// обновление без значения шифртекст не трогает
	require.NoError(t, s.itemUsecase.Update(ctx, itemId, &itemModel.Edit{Description: new("db password")}))
	assert.Equal(t, storedAfterUpdate, s.rawValue(t, itemId))

	// одинаковые значения разных item-ов дают разные шифртексты
	twinId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("PG_PASSWORD_COPY"), Value: new(newValue),
	})
	require.NoError(t, err)
	assert.NotEqual(t, storedAfterUpdate, s.rawValue(t, twinId))

	// пустое значение остаётся пустым: проверки на пустоту в SQL работают
	emptyId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("EMPTY"), Value: new(""),
	})
	require.NoError(t, err)
	assert.Empty(t, s.rawValue(t, emptyId))
	require.NoError(t, s.itemUsecase.Update(ctx, twinId, &itemModel.Edit{Value: new("")}))
	assert.Empty(t, s.rawValue(t, twinId))
	assert.EqualValues(t, 2, s.count(t, `select count(*) from item where value = ''`))

	// значение, похожее на шифртекст, при включённом шифровании — обычное значение
	const lookalike = secretbox.Prefix + "not-a-ciphertext"
	lookalikeId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("LOOKALIKE"), Value: new(lookalike),
	})
	require.NoError(t, err)
	assert.NotEqual(t, lookalike, s.rawValue(t, lookalikeId))
	got, err = s.itemUsecase.Get(ctx, lookalikeId)
	require.NoError(t, err)
	assert.Equal(t, lookalike, got.Value)
}

// Повреждённый шифртекст — ошибка чтения, а не «значение»: ни шифртекст, ни
// мусор не должны уйти в ответ API или в кластер.
func TestCorruptedCiphertextIsAnError(t *testing.T) {
	s := newStackWith(t, isolatedDSN(t), encBox(t, 1))
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "enc-corrupted")
	itemId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("TOKEN"), Value: new("corrupted-secret-value"),
	})
	require.NoError(t, err)

	// портим шифртекст в базе: отрезаем хвост (тег подлинности)
	_, err = s.pool.Exec(context.Background(),
		`update item set value = left(value, length(value) - 8) where id = $1`, itemId)
	require.NoError(t, err)

	_, err = s.itemUsecase.Get(ctx, itemId)
	require.ErrorIs(t, err, secretbox.ErrDecrypt)
	assert.Contains(t, err.Error(), itemId)

	_, _, err = s.itemUsecase.List(ctx, &itemModel.ListReq{SecretId: &secretId})
	require.ErrorIs(t, err, secretbox.ErrDecrypt)

	// и поиск по значению не проглатывает ошибку
	_, _, err = s.itemUsecase.List(ctx, &itemModel.ListReq{SecretId: &secretId, Search: new("corrupted")})
	require.ErrorIs(t, err, secretbox.ErrDecrypt)
}

// Перевод старых открытых строк при старте: шифруются все непустые значения
// (несколько пачек), updated_at и аудит не меняются, значения читаются
// прежними. Повторный перевод ничего не делает.
func TestApplyValueEncryptionSealsLegacyValues(t *testing.T) {
	dsn := isolatedDSN(t)
	ctx := context.Background()

	// «старая» база: сервис без ключа, значения открытым текстом
	plain := newStackWith(t, dsn, noBox(t))
	_, secretId := plain.newApp(t, "enc-migrate")

	// 1203 строки — три пачки по 500
	const legacyCount = 1203
	_, err := plain.pool.Exec(ctx, `
		insert into item (secret_id, key, value, updated_at)
		select $1, 'LEGACY_' || lpad(n::text, 5, '0'), 'legacy-value-' || n, now() - interval '30 days'
		from generate_series(1, $2::int) n
	`, secretId, legacyCount)
	require.NoError(t, err)

	special := map[string]string{
		"EMPTY":     "",
		"UNICODE":   "пароль ✓ с юникодом",
		"MULTILINE": "-----BEGIN KEY-----\nabc\ndef\n-----END KEY-----\n",
		"LARGE":     strings.Repeat("0123456789abcdef", 4096),
	}
	for key, value := range special {
		_, err = plain.itemUsecase.Create(plain.adminCtx(), &itemModel.Edit{
			SecretId: &secretId, Active: new(true), Key: new(key), Value: new(value),
		})
		require.NoError(t, err)
	}
	const nonEmptyCount = legacyCount + 3

	// без ключа и без зашифрованных строк старт проходит и ничего не меняет
	sealedCount, err := plain.itemSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.Zero(t, sealedCount)

	before := plain.rawItems(t)
	require.Len(t, before, legacyCount+len(special))
	auditBefore := plain.count(t, `select count(*) from audit`)
	require.EqualValues(t, len(special), auditBefore)

	// сервис поднимается с ключом
	sealed := newStackWith(t, dsn, encBox(t, 1))

	sealedCount, err = sealed.itemSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, nonEmptyCount, sealedCount)

	after := sealed.rawItems(t)
	require.Len(t, after, len(before))
	for id, old := range before {
		cur := after[id]
		if old.value == "" {
			assert.Empty(t, cur.value)
		} else {
			require.True(t, strings.HasPrefix(cur.value, secretbox.Prefix), "item %s is not sealed", id)
			require.NotEqual(t, old.value, cur.value)
		}
		// updated_at не тронут: содержимое не менялось
		require.True(t, old.updatedAt.Equal(cur.updatedAt), "item %s: updated_at changed", id)
	}
	assert.Zero(t, sealed.count(t,
		`select count(*) from item where value <> '' and not starts_with(value, $1)`, secretbox.Prefix))
	// записей аудита не прибавилось
	assert.Equal(t, auditBefore, sealed.count(t, `select count(*) from audit`))

	// значения читаются прежними
	items, _, err := sealed.itemSvc.List(ctx, &itemModel.ListReq{SecretId: &secretId})
	require.NoError(t, err)
	require.Len(t, items, len(before))
	for _, item := range items {
		require.Equal(t, before[item.Id].value, item.Value, "item %s (%s)", item.Id, item.Key)
	}

	// повторный перевод ничего не меняет
	sealedCount, err = sealed.itemSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.Zero(t, sealedCount)
	assert.Equal(t, after, sealed.rawItems(t))

	// строка, записанная открытым текстом позже (экземпляр без ключа, ещё не
	// остановленный при выкладке), шифруется при следующем старте
	_, err = sealed.pool.Exec(ctx,
		`insert into item (secret_id, key, value) values ($1, 'LATE_PLAIN', 'late-plain-value')`, secretId)
	require.NoError(t, err)

	sealedCount, err = sealed.itemSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, sealedCount)

	late, _, err := sealed.itemSvc.List(ctx, &itemModel.ListReq{SecretId: &secretId, Search: new("LATE_PLAIN")})
	require.NoError(t, err)
	require.Len(t, late, 1)
	assert.Equal(t, "late-plain-value", late[0].Value)
	assert.True(t, strings.HasPrefix(sealed.rawValue(t, late[0].Id), secretbox.Prefix))
}

// Старт без ключа при зашифрованных строках — ошибка: иначе сервис отдавал бы
// шифртекст вместо значений (в том числе в кластер при sync).
func TestStartWithoutKeyFailsOnEncryptedValues(t *testing.T) {
	dsn := isolatedDSN(t)
	ctx := context.Background()

	sealed := newStackWith(t, dsn, encBox(t, 1))
	_, secretId := sealed.newApp(t, "enc-no-key")

	const secretValue = "No-Key-Secret-Value"
	itemIds := make([]string, 0, 3)
	for _, key := range []string{"K1", "K2", "K3"} {
		itemId, err := sealed.itemUsecase.Create(sealed.adminCtx(), &itemModel.Edit{
			SecretId: &secretId, Active: new(true), Key: new(key), Value: new(secretValue),
		})
		require.NoError(t, err)
		itemIds = append(itemIds, itemId)
	}

	// тот же сервис без ключа
	plain := newStackWith(t, dsn, noBox(t))

	sealedCount, err := plain.itemSvc.ApplyValueEncryption(ctx)
	require.ErrorIs(t, err, secretbox.ErrKeyRequired)
	assert.Zero(t, sealedCount)
	assert.Contains(t, err.Error(), "DATA_ENCRYPTION_KEY")
	assert.Contains(t, err.Error(), "3 item values are encrypted")

	// и чтение без ключа — ошибка, а не шифртекст вместо значения
	_, err = plain.itemUsecase.Get(plain.adminCtx(), itemIds[0])
	require.ErrorIs(t, err, secretbox.ErrKeyRequired)
	_, _, err = plain.itemSvc.List(ctx, &itemModel.ListReq{SecretId: &secretId})
	require.ErrorIs(t, err, secretbox.ErrKeyRequired)

	// с ключом тот же старт проходит
	sealedCount, err = sealed.itemSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.Zero(t, sealedCount)
}

// Старт с чужим ключом — ошибка до любых изменений: иначе новые значения
// писались бы вторым ключом вперемешку со старыми.
func TestStartWithWrongKeyFails(t *testing.T) {
	dsn := isolatedDSN(t)
	ctx := context.Background()

	sealed := newStackWith(t, dsn, encBox(t, 1))
	_, secretId := sealed.newApp(t, "enc-wrong-key")

	_, err := sealed.itemUsecase.Create(sealed.adminCtx(), &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("SEALED"), Value: new("sealed-with-key-1"),
	})
	require.NoError(t, err)
	// и одна ещё не переведённая открытая строка
	_, err = sealed.pool.Exec(ctx,
		`insert into item (secret_id, key, value) values ($1, 'PLAIN', 'still-plain-value')`, secretId)
	require.NoError(t, err)

	before := sealed.rawItems(t)

	other := newStackWith(t, dsn, encBox(t, 2))
	sealedCount, err := other.itemSvc.ApplyValueEncryption(ctx)
	require.ErrorIs(t, err, secretbox.ErrDecrypt)
	assert.Zero(t, sealedCount)
	assert.Contains(t, err.Error(), "DATA_ENCRYPTION_KEY does not match")

	// чужим ключом ничего не зашифровано
	assert.Equal(t, before, other.rawItems(t))

	// с правильным ключом открытая строка переводится
	sealedCount, err = sealed.itemSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, sealedCount)
}

// Без ключа значение с зарезервированным префиксом сохранить нельзя: оно
// выглядело бы зашифрованным, и сервис не поднялся бы при следующем старте.
func TestReservedPrefixIsRejectedWithoutKey(t *testing.T) {
	s := newStackWith(t, isolatedDSN(t), noBox(t))
	ctx := s.adminCtx()

	_, secretId := s.newApp(t, "enc-prefix")

	requireInvalid := func(err error) {
		t.Helper()
		require.Error(t, err)
		fullErr, ok := errors.AsType[errs.ErrFull](err)
		require.True(t, ok, "expected errs.ErrFull, got: %v", err)
		assert.Equal(t, errs.InvalidRequest, fullErr.Err)
		assert.Equal(t, `item value must not start with "enc:v1:"`, fullErr.Desc)
	}

	_, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("LOOKALIKE"), Value: new(secretbox.Prefix + "anything"),
	})
	requireInvalid(err)

	itemId, err := s.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &secretId, Active: new(true), Key: new("NORMAL"), Value: new("normal-value"),
	})
	require.NoError(t, err)
	requireInvalid(s.itemUsecase.Update(ctx, itemId, &itemModel.Edit{Value: new(secretbox.Prefix + "anything")}))

	// значение не изменилось и лежит открытым текстом, как и раньше
	assert.Equal(t, "normal-value", s.rawValue(t, itemId))
	sealedCount, err := s.itemSvc.ApplyValueEncryption(context.Background())
	require.NoError(t, err)
	assert.Zero(t, sealedCount)
}

// ── Поиск по значению ───────────────────────────────────

type searchFixture struct {
	appA, secretA string
	appB, secretB string
}

// seedSearchData — один и тот же набор item-ов для сравнения поиска с
// шифрованием и без. Слово «zebra» встречается в значении, ключе или описании:
// в secretA — 32 совпадения (25 по значению, 3 по ключу, 2 по описанию,
// 2 выключенных по значению), в secretB — ещё 5 по значению.
func seedSearchData(t *testing.T, s *stack) searchFixture {
	t.Helper()
	ctx := context.Background()

	var f searchFixture
	f.appA, f.secretA = s.newApp(t, "search-a")
	f.appB, f.secretB = s.newApp(t, "search-b")

	add := func(secretId, key, value, description string, active bool) {
		_, err := s.itemSvc.Create(ctx, &itemModel.Edit{
			SecretId: &secretId, Active: &active, Key: &key, Value: &value, Description: &description,
		})
		require.NoError(t, err)
	}

	// совпадение только по значению (регистр в значении — смешанный)
	for i := range 25 {
		add(f.secretA, fmt.Sprintf("VAL_%02d", i), fmt.Sprintf("prefix-Zebra-%02d-suffix", i), "", true)
	}
	// по ключу и по описанию
	for i := range 3 {
		add(f.secretA, fmt.Sprintf("ZEBRA_KEY_%d", i), "plain", "", true)
	}
	for i := range 2 {
		add(f.secretA, fmt.Sprintf("DESC_%d", i), "other", "about zebra crossing", true)
	}
	// выключенные, совпадение по значению
	for i := range 2 {
		add(f.secretA, fmt.Sprintf("OFF_%d", i), "off-ZEBRA-value", "", false)
	}
	// не совпадают
	for i := range 10 {
		add(f.secretA, fmt.Sprintf("OTHER_%02d", i), "nothing-here", "", true)
	}
	add(f.secretA, "EMPTY", "", "", true)

	// второе приложение
	for i := range 5 {
		add(f.secretB, fmt.Sprintf("B_VAL_%d", i), "b-zebra-value", "", true)
	}

	return f
}

// Поиск по значению при включённом шифровании идёт в памяти, после
// расшифровки. Он обязан находить то же, что SQL-поиск без шифрования, и так
// же считать страницы и общее число.
func TestItemSearchByValueWithEncryption(t *testing.T) {
	plain := newStackWith(t, isolatedDSN(t), noBox(t))
	sealed := newStackWith(t, isolatedDSN(t), encBox(t, 1))

	plainData := seedSearchData(t, plain)
	sealedData := seedSearchData(t, sealed)

	// в базе с шифрованием слова «zebra» в значениях нет — SQL его не найдёт
	require.EqualValues(t, 32, plain.count(t, `select count(*) from item where value ilike '%zebra%'`))
	require.Zero(t, sealed.count(t, `select count(*) from item where value ilike '%zebra%'`))

	const (
		scopeSecretA = iota
		scopeAppA
		scopeGlobal
	)
	type searchCase struct {
		name      string
		scope     int
		needle    string
		list      commonModel.ListParams
		active    *bool
		wantLen   int
		wantTotal int64
	}

	search := func(s *stack, f searchFixture, c searchCase) ([]string, int64) {
		t.Helper()
		pars := &itemModel.ListReq{ListParams: c.list, Search: &c.needle, Active: c.active}
		switch c.scope {
		case scopeSecretA:
			pars.SecretId = &f.secretA
		case scopeAppA:
			pars.AppId = &f.appA
		}
		items, total, err := s.itemUsecase.List(s.adminCtx(), pars)
		require.NoError(t, err, c.name)
		return lo.Map(items, func(item *itemModel.Main, _ int) string { return item.Key }), total
	}

	page := func(page, size int64) commonModel.ListParams {
		return commonModel.ListParams{Page: page, PageSize: size, WithTotalCount: true}
	}

	cases := []searchCase{
		{name: "page 0", scope: scopeSecretA, needle: "zebra", list: page(0, 10), wantLen: 10, wantTotal: 32},
		{name: "page 1", scope: scopeSecretA, needle: "zebra", list: page(1, 10), wantLen: 10, wantTotal: 32},
		{name: "page 2", scope: scopeSecretA, needle: "zebra", list: page(2, 10), wantLen: 10, wantTotal: 32},
		{name: "last page", scope: scopeSecretA, needle: "zebra", list: page(3, 10), wantLen: 2, wantTotal: 32},
		{name: "page after last", scope: scopeSecretA, needle: "zebra", list: page(4, 10), wantLen: 0, wantTotal: 32},
		{name: "upper case needle", scope: scopeSecretA, needle: "ZEBRA", list: page(0, 100), wantLen: 32, wantTotal: 32},
		{name: "no pagination", scope: scopeSecretA, needle: "zebra", wantLen: 32},
		{
			name: "page without total", scope: scopeSecretA, needle: "zebra",
			list: commonModel.ListParams{Page: 1, PageSize: 10}, wantLen: 10,
		},
		{
			name: "only count", scope: scopeSecretA, needle: "zebra",
			list: commonModel.ListParams{OnlyCount: true}, wantTotal: 32,
		},
		{
			name: "sort desc", scope: scopeSecretA, needle: "zebra",
			list:    commonModel.ListParams{PageSize: 10, WithTotalCount: true, Sort: []string{"-key"}},
			wantLen: 10, wantTotal: 32,
		},
		{name: "by app", scope: scopeAppA, needle: "zebra", list: page(0, 50), wantLen: 32, wantTotal: 32},
		{name: "global", scope: scopeGlobal, needle: "zebra", list: page(0, 50), wantLen: 37, wantTotal: 37},
		{name: "global last page", scope: scopeGlobal, needle: "zebra", list: page(3, 10), wantLen: 7, wantTotal: 37},
		{
			name: "active only", scope: scopeSecretA, needle: "zebra", list: page(0, 50),
			active: new(true), wantLen: 30, wantTotal: 30,
		},
		{
			name: "inactive only", scope: scopeSecretA, needle: "zebra", list: page(0, 50),
			active: new(false), wantLen: 2, wantTotal: 2,
		},
		{name: "value fragment", scope: scopeSecretA, needle: "zebra-1", list: page(0, 50), wantLen: 10, wantTotal: 10},
		// спецсимволы LIKE работают так же, как в SQL
		{name: "like one char", scope: scopeSecretA, needle: "zebra_0", list: page(0, 50), wantLen: 10, wantTotal: 10},
		{name: "like any chars", scope: scopeSecretA, needle: "prefix%07", list: page(0, 50), wantLen: 1, wantTotal: 1},
		{name: "by key", scope: scopeSecretA, needle: "zebra_key", list: page(0, 50), wantLen: 3, wantTotal: 3},
		{name: "by description", scope: scopeSecretA, needle: "crossing", list: page(0, 50), wantLen: 2, wantTotal: 2},
		{name: "no match", scope: scopeSecretA, needle: "no-such-value-xyz", list: page(0, 10)},
	}
	for _, c := range cases {
		plainKeys, plainTotal := search(plain, plainData, c)
		sealedKeys, sealedTotal := search(sealed, sealedData, c)

		assert.Len(t, sealedKeys, c.wantLen, c.name)
		assert.Equal(t, c.wantTotal, sealedTotal, c.name)
		// тот же состав, тот же порядок и то же общее число, что и без шифрования
		assert.Equal(t, plainKeys, sealedKeys, c.name)
		assert.Equal(t, plainTotal, sealedTotal, c.name)
	}

	// страницы не пересекаются и вместе дают весь список в том же порядке
	whole, _ := search(sealed, sealedData, searchCase{name: "whole", scope: scopeSecretA, needle: "zebra"})
	require.Len(t, whole, 32)
	paged := make([]string, 0, len(whole))
	for pageNum := range int64(4) {
		keys, _ := search(sealed, sealedData, searchCase{
			name: "paged", scope: scopeSecretA, needle: "zebra", list: page(pageNum, 10),
		})
		paged = append(paged, keys...)
	}
	assert.Equal(t, whole, paged)
	assert.Len(t, lo.Uniq(paged), 32)
	assert.Equal(t, 25, lo.CountBy(paged, func(key string) bool { return strings.HasPrefix(key, "VAL_") }))

	// обратная сортировка — те же ключи в обратном порядке
	desc, _ := search(sealed, sealedData, searchCase{
		name: "desc", scope: scopeSecretA, needle: "zebra", list: commonModel.ListParams{Sort: []string{"-key"}},
	})
	slices.Reverse(desc)
	assert.Equal(t, whole, desc)

	// найденные значения возвращаются расшифрованными
	items, _, err := sealed.itemUsecase.List(sealed.adminCtx(), &itemModel.ListReq{
		SecretId: &sealedData.secretB, Search: new("B-ZEBRA"),
	})
	require.NoError(t, err)
	require.Len(t, items, 5)
	for _, item := range items {
		assert.Equal(t, "b-zebra-value", item.Value)
	}

	// сессия, которой значения не видны, по значению не ищет — и с шифрованием тоже
	roCtx := sealed.sessionSvc.WithContext(context.Background(), &sessionModel.Session{
		Id: 1, Admin: true, Name: "RO", Scope: "read_only", Source: "api",
	})
	hidden, total, err := sealed.itemUsecase.List(roCtx, &itemModel.ListReq{
		ListParams: page(0, 50), SecretId: &sealedData.secretA, Search: new("zebra"),
	})
	require.NoError(t, err)
	assert.EqualValues(t, 5, total) // 3 по ключу + 2 по описанию
	assert.Len(t, hidden, 5)
	for _, item := range hidden {
		assert.Empty(t, item.Value)
	}
}
