package audit

import (
	"strconv"
	"strings"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/util"
)

// configValueMaxLen — лимит несекретного значения (config_item) в audit.changes;
// длиннее — сохраняются только отпечаток и размер (Truncated).
const configValueMaxLen = 4096

// maskedMarker — заглушка для полей, чьи значения не пишутся в аудит даже
// отпечатком (пароль, TOTP-секрет): фиксируется только факт изменения.
const maskedMarker = "***"

// changeSet — построитель списка изменённых полей: на create пишутся
// new-стороны, на delete — old-стороны, на update — только различия.
type changeSet struct {
	hasOld, hasNew bool
	changes        []auditModel.Change
}

func newChangeSet(hasOld, hasNew bool) *changeSet {
	return &changeSet{hasOld: hasOld, hasNew: hasNew}
}

func (c *changeSet) str(field, oldV, newV string) {
	switch {
	case c.hasOld && c.hasNew:
		if oldV != newV {
			c.changes = append(c.changes, auditModel.Change{Field: field, Old: &oldV, New: &newV})
		}
	case c.hasNew:
		if newV != "" {
			c.changes = append(c.changes, auditModel.Change{Field: field, New: &newV})
		}
	case c.hasOld:
		if oldV != "" {
			c.changes = append(c.changes, auditModel.Change{Field: field, Old: &oldV})
		}
	}
}

// boolF пишет булево поле; на create/delete — всегда (default тоже информативен).
func (c *changeSet) boolF(field string, oldV, newV bool) {
	oldS, newS := strconv.FormatBool(oldV), strconv.FormatBool(newV)
	switch {
	case c.hasOld && c.hasNew:
		if oldV != newV {
			c.changes = append(c.changes, auditModel.Change{Field: field, Old: &oldS, New: &newS})
		}
	case c.hasNew:
		c.changes = append(c.changes, auditModel.Change{Field: field, New: &newS})
	case c.hasOld:
		c.changes = append(c.changes, auditModel.Change{Field: field, Old: &oldS})
	}
}

func (c *changeSet) int64F(field string, oldV, newV int64) {
	c.str(field, strconv.FormatInt(oldV, 10), strconv.FormatInt(newV, 10))
}

func (c *changeSet) strSlice(field string, oldV, newV []string) {
	c.str(field, strings.Join(oldV, ", "), strings.Join(newV, ", "))
}

// masked фиксирует изменение поля без значения и без отпечатка (пароль,
// TOTP-секрет). На create/delete не пишется — важен только факт смены.
func (c *changeSet) masked(field string, changed bool) {
	if !c.hasOld || !c.hasNew || !changed {
		return
	}
	oldS, newS := maskedMarker, maskedMarker
	c.changes = append(c.changes, auditModel.Change{Field: field, Old: &oldS, New: &newS})
}

// fingerprintChange — изменение значения только отпечатком и размером.
func (c *changeSet) fingerprintChange(field, hashKey, oldV, newV string, truncated bool) {
	change := auditModel.Change{Field: field, Truncated: truncated}
	if c.hasOld {
		change.OldHash = util.ValueFingerprint(hashKey, oldV)
		change.OldSize = new(int64(len(oldV)))
	}
	if c.hasNew {
		change.NewHash = util.ValueFingerprint(hashKey, newV)
		change.NewSize = new(int64(len(newV)))
	}
	c.changes = append(c.changes, change)
}

// secretValue — значение item (secret): в аудит попадают только отпечаток
// и размер, само значение — никогда.
func (c *changeSet) secretValue(field, hashKey, oldV, newV string) {
	if c.hasOld && c.hasNew && oldV == newV {
		return
	}
	c.fingerprintChange(field, hashKey, oldV, newV, false)
}

// configValue — значение config_item (несекретное): old/new целиком, но
// значения длиннее configValueMaxLen заменяются отпечатком и размером.
func (c *changeSet) configValue(field, hashKey, oldV, newV string) {
	if c.hasOld && c.hasNew && oldV == newV {
		return
	}
	if len(oldV) > configValueMaxLen || len(newV) > configValueMaxLen {
		c.fingerprintChange(field, hashKey, oldV, newV, true)
		return
	}
	c.str(field, oldV, newV)
}

// ── Диффы по сущностям ──────────────────────────────────

func appChanges(old, cur *appModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n appModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.str("namespace", o.Namespace, n.Namespace)
	c.str("name", o.Name, n.Name)
	c.str("slug_name", o.SlugName, n.SlugName)
	c.str("description", o.Description, n.Description)
	return c.changes
}

func secretChanges(old, cur *secretModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n secretModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.str("app_id", o.AppId, n.AppId)
	c.str("slug_name", o.SlugName, n.SlugName)
	c.str("description", o.Description, n.Description)
	c.str("kube_type", o.KubeType, n.KubeType)
	c.boolF("exact_slug", o.ExactSlug, n.ExactSlug)
	return c.changes
}

func (s *Service) itemChanges(old, cur *itemModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n itemModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.str("secret_id", o.SecretId, n.SecretId)
	c.str("key", o.Key, n.Key)
	c.secretValue("value", s.hashKey, o.Value, n.Value)
	c.str("value_format", o.ValueFormat, n.ValueFormat)
	c.str("encoding", o.Encoding, n.Encoding)
	c.str("file_name", o.FileName, n.FileName)
	c.str("content_type", o.ContentType, n.ContentType)
	c.str("description", o.Description, n.Description)
	return c.changes
}

func configMapChanges(old, cur *configmapModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n configmapModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.str("app_id", o.AppId, n.AppId)
	c.str("slug_name", o.SlugName, n.SlugName)
	c.str("description", o.Description, n.Description)
	c.boolF("exact_slug", o.ExactSlug, n.ExactSlug)
	return c.changes
}

func (s *Service) configItemChanges(old, cur *configitemModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n configitemModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.str("configmap_id", o.ConfigMapId, n.ConfigMapId)
	c.str("key", o.Key, n.Key)
	c.configValue("value", s.hashKey, o.Value, n.Value)
	c.str("value_format", o.ValueFormat, n.ValueFormat)
	c.str("encoding", o.Encoding, n.Encoding)
	c.str("file_name", o.FileName, n.FileName)
	c.str("content_type", o.ContentType, n.ContentType)
	c.str("description", o.Description, n.Description)
	return c.changes
}

// usrChanges — password и totp_secret не пишутся ни значением, ни отпечатком.
func usrChanges(old, cur *usrModel.Main) []auditModel.Change {
	c := newChangeSet(old != nil, cur != nil)
	var o, n usrModel.Main
	if old != nil {
		o = *old
	}
	if cur != nil {
		n = *cur
	}
	c.boolF("active", o.Active, n.Active)
	c.boolF("is_admin", o.IsAdmin, n.IsAdmin)
	c.str("name", o.Name, n.Name)
	c.str("username", o.Username, n.Username)
	c.strSlice("app_ids", o.AppIds, n.AppIds)
	c.boolF("totp_enabled", o.TotpEnabled, n.TotpEnabled)
	c.masked("password", o.Password != n.Password)
	return c.changes
}
