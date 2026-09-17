package audit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/util"
)

const testHashKey = "test-hash-key"

func testService() *Service {
	return &Service{hashKey: testHashKey}
}

// changesJSON — сериализация диффа так же, как он попадает в jsonb-колонку.
func changesJSON(t *testing.T, changes []auditModel.Change) string {
	t.Helper()
	raw, err := json.Marshal(changes)
	require.NoError(t, err)
	return string(raw)
}

// Значения item (secret) не должны попадать в запись аудита ни в одном поле —
// ни на create, ни на update, ни на delete.
func TestItemChangesSecretValueNeverLeaks(t *testing.T) {
	s := testService()

	oldValue := "SUPER-SECRET-OLD-VALUE-9f8e7d"
	newValue := "SUPER-SECRET-NEW-VALUE-1a2b3c"

	oldItem := &itemModel.Main{Id: "i1", SecretId: "s1", Key: "PG_PASSWORD", Value: oldValue, Description: "d"}
	newItem := &itemModel.Main{Id: "i1", SecretId: "s1", Key: "PG_PASSWORD", Value: newValue, Description: "d2"}

	for name, changes := range map[string][]auditModel.Change{
		"create": s.itemChanges(nil, newItem),
		"update": s.itemChanges(oldItem, newItem),
		"delete": s.itemChanges(oldItem, nil),
	} {
		raw := changesJSON(t, changes)
		assert.NotContains(t, raw, oldValue, name)
		assert.NotContains(t, raw, newValue, name)
	}
}

func TestItemChangesValueFingerprint(t *testing.T) {
	s := testService()

	oldItem := &itemModel.Main{Id: "i1", Key: "K", Value: "old-value"}
	newItem := &itemModel.Main{Id: "i1", Key: "K", Value: "new-value"}

	changes := s.itemChanges(oldItem, newItem)
	require.Len(t, changes, 1)

	change := changes[0]
	assert.Equal(t, "value", change.Field)
	assert.Nil(t, change.Old)
	assert.Nil(t, change.New)
	assert.Equal(t, util.ValueFingerprint(testHashKey, "old-value"), change.OldHash)
	assert.Equal(t, util.ValueFingerprint(testHashKey, "new-value"), change.NewHash)
	require.NotNil(t, change.OldSize)
	require.NotNil(t, change.NewSize)
	assert.Equal(t, int64(len("old-value")), *change.OldSize)
	assert.Equal(t, int64(len("new-value")), *change.NewSize)

	// одинаковое значение — изменения нет
	same := &itemModel.Main{Id: "i1", Key: "K", Value: "old-value"}
	assert.Empty(t, s.itemChanges(oldItem, same))

	// «вернулось к прежнему» видно по совпадению отпечатков
	back := s.itemChanges(newItem, oldItem)
	require.Len(t, back, 1)
	assert.Equal(t, change.NewHash, back[0].OldHash)
	assert.Equal(t, change.OldHash, back[0].NewHash)
}

func configItem(value string) configitemModel.Main {
	return configitemModel.Main{Id: "ci1", ConfigMapId: "cm1", Key: "K", Value: value}
}

func TestConfigItemChangesValueStoredWithLimit(t *testing.T) {
	s := testService()

	shortOldV, shortNewV := configItem("short-old"), configItem("short-new")
	shortOld, shortNew := &shortOldV, &shortNewV

	changes := s.configItemChanges(shortOld, shortNew)
	require.Len(t, changes, 1)
	require.NotNil(t, changes[0].Old)
	require.NotNil(t, changes[0].New)
	assert.Equal(t, "short-old", *changes[0].Old)
	assert.Equal(t, "short-new", *changes[0].New)
	assert.False(t, changes[0].Truncated)

	// длинное значение — только отпечаток и размер
	long := configItem(strings.Repeat("x", configValueMaxLen+1))
	changes = s.configItemChanges(shortOld, &long)
	require.Len(t, changes, 1)
	assert.Nil(t, changes[0].Old)
	assert.Nil(t, changes[0].New)
	assert.True(t, changes[0].Truncated)
	assert.NotEmpty(t, changes[0].OldHash)
	assert.NotEmpty(t, changes[0].NewHash)
	require.NotNil(t, changes[0].NewSize)
	assert.Equal(t, int64(configValueMaxLen+1), *changes[0].NewSize)
}

func TestUsrChangesCredentialsMasked(t *testing.T) {
	oldUsr := &usrModel.Main{Id: 1, Name: "N", Username: "u", Password: "hash-old", TotpSecret: "totp-secret-old"}
	newUsr := &usrModel.Main{Id: 1, Name: "N", Username: "u", Password: "hash-new", TotpSecret: "totp-secret-new"}

	for name, changes := range map[string][]auditModel.Change{
		"create": usrChanges(nil, newUsr),
		"update": usrChanges(oldUsr, newUsr),
		"delete": usrChanges(oldUsr, nil),
	} {
		raw := changesJSON(t, changes)
		assert.NotContains(t, raw, "hash-old", name)
		assert.NotContains(t, raw, "hash-new", name)
		assert.NotContains(t, raw, "totp-secret", name)
	}

	// факт смены пароля фиксируется маской
	changes := usrChanges(oldUsr, newUsr)
	require.Len(t, changes, 1)
	assert.Equal(t, "password", changes[0].Field)
	require.NotNil(t, changes[0].New)
	assert.Equal(t, maskedMarker, *changes[0].New)
}

func TestDeriveAction(t *testing.T) {
	activeTrue, activeFalse := "true", "false"

	assert.Equal(t, auditModel.ActionCreate, deriveAction(false, true, nil))
	assert.Equal(t, auditModel.ActionDelete, deriveAction(true, false, nil))
	assert.Equal(t, auditModel.ActionUpdate, deriveAction(true, true, []auditModel.Change{
		{Field: "description"},
	}))
	assert.Equal(t, auditModel.ActionActivate, deriveAction(true, true, []auditModel.Change{
		{Field: "active", Old: &activeFalse, New: &activeTrue},
	}))
	assert.Equal(t, auditModel.ActionDeactivate, deriveAction(true, true, []auditModel.Change{
		{Field: "active", Old: &activeTrue, New: &activeFalse},
	}))
	// active в составе прочих изменений — обычный update
	assert.Equal(t, auditModel.ActionUpdate, deriveAction(true, true, []auditModel.Change{
		{Field: "active", Old: &activeTrue, New: &activeFalse},
		{Field: "description"},
	}))
}

func TestFingerprint(t *testing.T) {
	fp := util.ValueFingerprint("key1", "value")

	assert.Len(t, fp, 16)
	// детерминированность и зависимость от ключа и значения
	assert.Equal(t, fp, util.ValueFingerprint("key1", "value"))
	assert.NotEqual(t, fp, util.ValueFingerprint("key2", "value"))
	assert.NotEqual(t, fp, util.ValueFingerprint("key1", "value2"))
}
