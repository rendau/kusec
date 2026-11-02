package integration

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/errs"
	inactiveUsc "github.com/rendau/kusec/internal/usecase/inactive"
)

type inactiveFixture struct {
	appId, activeSecretId, offSecretId  string
	offItemId, keptItemId, nestedItemId string
	offConfigMapId, offConfigItemId     string
}

// newInactiveFixture: активный секрет с одним выключенным ключом, выключенный
// секрет с активным ключом внутри, выключенный конфигмап с выключенным ключом.
func (s *stack) newInactiveFixture(t *testing.T, slug string) inactiveFixture {
	t.Helper()
	ctx := context.Background()

	var f inactiveFixture
	f.appId, f.activeSecretId = s.newApp(t, slug)

	newItem := func(secretId, key string, active bool) string {
		id, err := s.itemSvc.Create(ctx, &itemModel.Edit{
			SecretId: &secretId, Active: &active, Key: &key, Value: new("v"),
		})
		require.NoError(t, err)
		return id
	}
	f.offItemId = newItem(f.activeSecretId, "OFF_ITEM", false)
	f.keptItemId = newItem(f.activeSecretId, "KEPT_ITEM", true)

	var err error
	f.offSecretId, err = s.secretSvc.Create(ctx, &secretModel.Edit{
		AppId: &f.appId, Active: new(false), SlugName: new("off"),
	})
	require.NoError(t, err)
	f.nestedItemId = newItem(f.offSecretId, "NESTED", true)

	f.offConfigMapId, err = s.configMapSvc.Create(ctx, &configmapModel.Edit{
		AppId: &f.appId, Active: new(false), SlugName: new("off-cm"),
	})
	require.NoError(t, err)
	f.offConfigItemId, err = s.configItemSvc.Create(ctx, &configitemModel.Edit{
		ConfigMapId: &f.offConfigMapId, Active: new(false), Key: new("OFF_CFG"), Value: new("v"),
	})
	require.NoError(t, err)

	return f
}

func TestInactiveList(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	f := s.newInactiveFixture(t, "inactive-list")
	other, _ := s.newApp(t, "inactive-list-other")

	elements, err := s.inactiveUsecase.List(ctx, &f.appId)
	require.NoError(t, err)

	byId := lo.KeyBy(elements, func(e *inactiveUsc.Element) string { return e.Id })
	require.Len(t, elements, 4)

	assert.Equal(t, inactiveUsc.KindSecret, byId[f.offSecretId].Kind)
	assert.EqualValues(t, 1, byId[f.offSecretId].ChildCount)

	item := byId[f.offItemId]
	require.NotNil(t, item, "ключ внутри активного секрета тоже в списке")
	assert.Equal(t, "main", item.ParentName)
	assert.Equal(t, f.appId, item.AppId)
	assert.Equal(t, "App inactive-list", item.AppName)

	assert.EqualValues(t, 1, byId[f.offConfigMapId].ChildCount)
	assert.Equal(t, "off-cm", byId[f.offConfigItemId].ParentName)
	assert.NotContains(t, byId, f.nestedItemId, "активный ключ выключенного секрета считается в ChildCount")

	// чужое приложение в выборку по app_id не попадает
	elements, err = s.inactiveUsecase.List(ctx, &other)
	require.NoError(t, err)
	assert.Empty(t, elements)

	// выключенное приложение попадает в список вместе с числом потомков
	_, err = s.pool.Exec(context.Background(), `update app set active = false where id = $1`, other)
	require.NoError(t, err)
	elements, err = s.inactiveUsecase.List(ctx, &other)
	require.NoError(t, err)
	require.Len(t, elements, 1)
	assert.Equal(t, inactiveUsc.KindApp, elements[0].Kind)
	assert.EqualValues(t, 1, elements[0].ChildCount)

	// у приложения считается всё вложенное, включая item-ы секретов и конфигмапов
	_, err = s.pool.Exec(context.Background(), `update app set active = false where id = $1`, f.appId)
	require.NoError(t, err)
	elements, err = s.inactiveUsecase.List(ctx, &f.appId)
	require.NoError(t, err)
	appElement, ok := lo.Find(elements, func(e *inactiveUsc.Element) bool { return e.Kind == inactiveUsc.KindApp })
	require.True(t, ok)
	assert.EqualValues(t, 7, appElement.ChildCount) // 2 секрета, 3 item-а, конфигмап и его item
}

func TestInactiveDelete(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	f := s.newInactiveFixture(t, "inactive-delete")

	// родитель и его потомок в одном наборе: потомок не должен потеряться;
	// активный NESTED выключенного секрета удалению не мешает
	err := s.inactiveUsecase.Delete(ctx, []inactiveUsc.Ref{
		{Kind: inactiveUsc.KindSecret, Id: f.offSecretId},
		{Kind: inactiveUsc.KindItem, Id: f.offItemId},
		{Kind: inactiveUsc.KindConfigMap, Id: f.offConfigMapId},
		{Kind: inactiveUsc.KindConfigItem, Id: f.offConfigItemId},
		{Kind: inactiveUsc.KindItem, Id: "00000000-0000-0000-0000-000000000000"}, // уже удалён кем-то
	})
	require.NoError(t, err)

	_, found, err := s.itemSvc.Get(ctx, f.keptItemId, false)
	require.NoError(t, err)
	assert.True(t, found, "активный ключ не тронут")
	_, found, err = s.secretSvc.Get(ctx, f.activeSecretId, false)
	require.NoError(t, err)
	assert.True(t, found, "активный секрет не тронут")

	// все удаления одной операции связаны batch_id, потомки каскада — тоже
	secretRows := s.auditRows(t, "secret", f.offSecretId)
	require.Len(t, secretRows, 1)
	assert.Equal(t, "delete", secretRows[0].action)
	assert.NotEmpty(t, secretRows[0].batchId)
	for _, row := range [][]auditRow{
		s.auditRows(t, "item", f.offItemId),
		s.auditRows(t, "item", f.nestedItemId),
		s.auditRows(t, "configmap", f.offConfigMapId),
		s.auditRows(t, "config_item", f.offConfigItemId),
	} {
		require.Len(t, row, 1)
		assert.Equal(t, secretRows[0].batchId, row[0].batchId)
	}
}

func TestInactiveDeleteApp(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	f := s.newInactiveFixture(t, "inactive-delete-app")
	require.NoError(t, s.appSvc.Update(ctx, f.appId, &appModel.Edit{Active: new(false)}))

	// активные потомки выключенного приложения удалению не мешают
	err := s.inactiveUsecase.Delete(ctx, []inactiveUsc.Ref{
		{Kind: inactiveUsc.KindApp, Id: f.appId},
		{Kind: inactiveUsc.KindItem, Id: f.offItemId},
	})
	require.NoError(t, err)

	_, found, err := s.appSvc.Get(ctx, f.appId, false)
	require.NoError(t, err)
	assert.False(t, found)
	require.Len(t, s.auditRows(t, "item", f.keptItemId), 1, "потомок каскада в аудите")
}

// Включённый после загрузки списка элемент отменяет всю операцию.
func TestInactiveDeleteRejectsActive(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	f := s.newInactiveFixture(t, "inactive-reject")

	err := s.inactiveUsecase.Delete(ctx, []inactiveUsc.Ref{
		{Kind: inactiveUsc.KindItem, Id: f.offItemId},
		{Kind: inactiveUsc.KindItem, Id: f.keptItemId},
	})
	require.ErrorIs(t, err, errs.ElementActive)

	_, found, err := s.itemSvc.Get(ctx, f.offItemId, false)
	require.NoError(t, err)
	assert.True(t, found, "откат: выключенный ключ из того же набора остался")
}

func TestInactiveRequiresAdmin(t *testing.T) {
	s := newStack(t)
	ctx := s.sessionSvc.WithContext(context.Background(), &sessionModel.Session{Id: 7, Name: "user"})

	_, err := s.inactiveUsecase.List(ctx, nil)
	require.ErrorIs(t, err, errs.NoPermission)

	err = s.inactiveUsecase.Delete(ctx, []inactiveUsc.Ref{{Kind: inactiveUsc.KindItem, Id: "x"}})
	require.ErrorIs(t, err, errs.NoPermission)
}
