package inactive

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
)

// deleteOrder — порядок удаления: сначала потомки, чтобы выбранный вместе с
// родителем item дошёл до собственной проверки, а не исчез в его каскаде.
var deleteOrder = []string{KindItem, KindConfigItem, KindSecret, KindConfigMap, KindApp}

// kindRank — порядок видов в списке: от приложения к его ключам.
var kindRank = map[string]int{
	KindApp:        0,
	KindSecret:     1,
	KindItem:       2,
	KindConfigMap:  3,
	KindConfigItem: 4,
}

// Usecase — обзор деактивированных элементов и их удаление (только админы).
type Usecase struct {
	appSvc        AppServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI
	cascadeSvc    CascadeServiceI
	sessionSvc    SessionServiceI
	txm           TransactionManagerI
	auditRec      AuditRecorderI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
	cascadeSvc CascadeServiceI,
	sessionSvc SessionServiceI,
	txm TransactionManagerI,
	auditRec AuditRecorderI,
) *Usecase {
	return &Usecase{
		appSvc:        appSvc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
		cascadeSvc:    cascadeSvc,
		sessionSvc:    sessionSvc,
		txm:           txm,
		auditRec:      auditRec,
	}
}

// List отдаёт элементы с active=false. Потомки деактивированного родителя, у
// которых флаг не снят, в список не входят — их показывает ChildCount родителя.
func (u *Usecase) List(ctx context.Context, appId *string) ([]*Element, error) {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, errs.NoPermission
	}

	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	appNames := lo.SliceToMap(apps, func(a *appModel.Main) (string, string) { return a.Id, a.Name })

	appElements, err := u.listApps(ctx, apps, appId)
	if err != nil {
		return nil, err
	}
	secretElements, err := u.listSecrets(ctx, appId, appNames)
	if err != nil {
		return nil, err
	}
	itemElements, err := u.listItems(ctx, appId, appNames)
	if err != nil {
		return nil, err
	}
	configMapElements, err := u.listConfigMaps(ctx, appId, appNames)
	if err != nil {
		return nil, err
	}
	configItemElements, err := u.listConfigItems(ctx, appId, appNames)
	if err != nil {
		return nil, err
	}

	result := slices.Concat(appElements, secretElements, itemElements, configMapElements, configItemElements)
	slices.SortFunc(result, func(a, b *Element) int {
		return cmp.Or(
			cmp.Compare(a.AppName, b.AppName),
			cmp.Compare(kindRank[a.Kind], kindRank[b.Kind]),
			cmp.Compare(a.ParentName, b.ParentName),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return result, nil
}

func (u *Usecase) listApps(ctx context.Context, apps []*appModel.Main, appId *string) ([]*Element, error) {
	inactive := lo.Filter(apps, func(a *appModel.Main, _ int) bool {
		return !a.Active && (appId == nil || a.Id == *appId)
	})
	if len(inactive) == 0 {
		return nil, nil
	}

	childCounts, err := u.appChildCounts(ctx, lo.Map(inactive, func(a *appModel.Main, _ int) string { return a.Id }))
	if err != nil {
		return nil, err
	}

	return lo.Map(inactive, func(a *appModel.Main, _ int) *Element {
		return &Element{
			Kind:        KindApp,
			Id:          a.Id,
			AppId:       a.Id,
			AppName:     a.Name,
			Name:        a.Name,
			Description: a.Description,
			UpdatedAt:   a.UpdatedAt,
			ChildCount:  childCounts[a.Id],
		}
	}), nil
}

// appChildCounts считает всё, что уйдёт вместе с приложением: секреты,
// конфигмапы и их item-ы.
func (u *Usecase) appChildCounts(ctx context.Context, appIds []string) (map[string]int64, error) {
	secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppIds: appIds})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List(children): %w", err)
	}
	configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppIds: appIds})
	if err != nil {
		return nil, fmt.Errorf("configMapSvc.List(children): %w", err)
	}

	counts := make(map[string]int64, len(appIds))
	for _, secret := range secrets {
		counts[secret.AppId]++
	}
	for _, configMap := range configMaps {
		counts[configMap.AppId]++
	}

	if len(secrets) > 0 {
		appBySecret := lo.SliceToMap(secrets, func(s *secretModel.Main) (string, string) { return s.Id, s.AppId })
		items, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{SecretIds: lo.Keys(appBySecret)})
		if err != nil {
			return nil, fmt.Errorf("itemSvc.List(children): %w", err)
		}
		for _, item := range items {
			counts[appBySecret[item.SecretId]]++
		}
	}
	if len(configMaps) > 0 {
		appByConfigMap := lo.SliceToMap(configMaps, func(c *configmapModel.Main) (string, string) { return c.Id, c.AppId })
		configItems, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{ConfigMapIds: lo.Keys(appByConfigMap)})
		if err != nil {
			return nil, fmt.Errorf("configItemSvc.List(children): %w", err)
		}
		for _, configItem := range configItems {
			counts[appByConfigMap[configItem.ConfigMapId]]++
		}
	}

	return counts, nil
}

func (u *Usecase) listSecrets(ctx context.Context, appId *string, appNames map[string]string) ([]*Element, error) {
	inactive, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: appId, Active: new(false)})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List: %w", err)
	}
	if len(inactive) == 0 {
		return nil, nil
	}

	items, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{
		SecretIds: lo.Map(inactive, func(s *secretModel.Main, _ int) string { return s.Id }),
	})
	if err != nil {
		return nil, fmt.Errorf("itemSvc.List(children): %w", err)
	}
	itemCounts := lo.CountValuesBy(items, func(i *itemModel.Main) string { return i.SecretId })

	return lo.Map(inactive, func(s *secretModel.Main, _ int) *Element {
		return &Element{
			Kind:        KindSecret,
			Id:          s.Id,
			AppId:       s.AppId,
			AppName:     appNames[s.AppId],
			Name:        s.SlugName,
			Description: s.Description,
			UpdatedAt:   s.UpdatedAt,
			ChildCount:  int64(itemCounts[s.Id]),
		}
	}), nil
}

func (u *Usecase) listItems(ctx context.Context, appId *string, appNames map[string]string) ([]*Element, error) {
	inactive, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{AppId: appId, Active: new(false)})
	if err != nil {
		return nil, fmt.Errorf("itemSvc.List: %w", err)
	}
	if len(inactive) == 0 {
		return nil, nil
	}

	parents, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{
		Ids: lo.Uniq(lo.Map(inactive, func(i *itemModel.Main, _ int) string { return i.SecretId })),
	})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List(parents): %w", err)
	}
	parentById := lo.KeyBy(parents, func(s *secretModel.Main) string { return s.Id })

	return lo.FilterMap(inactive, func(i *itemModel.Main, _ int) (*Element, bool) {
		parent, ok := parentById[i.SecretId]
		if !ok {
			return nil, false
		}
		return &Element{
			Kind:        KindItem,
			Id:          i.Id,
			AppId:       parent.AppId,
			AppName:     appNames[parent.AppId],
			ParentId:    parent.Id,
			ParentName:  parent.SlugName,
			Name:        i.Key,
			Description: i.Description,
			UpdatedAt:   i.UpdatedAt,
		}, true
	}), nil
}

func (u *Usecase) listConfigMaps(ctx context.Context, appId *string, appNames map[string]string) ([]*Element, error) {
	inactive, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: appId, Active: new(false)})
	if err != nil {
		return nil, fmt.Errorf("configMapSvc.List: %w", err)
	}
	if len(inactive) == 0 {
		return nil, nil
	}

	configItems, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{
		ConfigMapIds: lo.Map(inactive, func(c *configmapModel.Main, _ int) string { return c.Id }),
	})
	if err != nil {
		return nil, fmt.Errorf("configItemSvc.List(children): %w", err)
	}
	itemCounts := lo.CountValuesBy(configItems, func(i *configitemModel.Main) string { return i.ConfigMapId })

	return lo.Map(inactive, func(c *configmapModel.Main, _ int) *Element {
		return &Element{
			Kind:        KindConfigMap,
			Id:          c.Id,
			AppId:       c.AppId,
			AppName:     appNames[c.AppId],
			Name:        c.SlugName,
			Description: c.Description,
			UpdatedAt:   c.UpdatedAt,
			ChildCount:  int64(itemCounts[c.Id]),
		}
	}), nil
}

func (u *Usecase) listConfigItems(ctx context.Context, appId *string, appNames map[string]string) ([]*Element, error) {
	inactive, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{AppId: appId, Active: new(false)})
	if err != nil {
		return nil, fmt.Errorf("configItemSvc.List: %w", err)
	}
	if len(inactive) == 0 {
		return nil, nil
	}

	parents, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{
		Ids: lo.Uniq(lo.Map(inactive, func(i *configitemModel.Main, _ int) string { return i.ConfigMapId })),
	})
	if err != nil {
		return nil, fmt.Errorf("configMapSvc.List(parents): %w", err)
	}
	parentById := lo.KeyBy(parents, func(c *configmapModel.Main) string { return c.Id })

	return lo.FilterMap(inactive, func(i *configitemModel.Main, _ int) (*Element, bool) {
		parent, ok := parentById[i.ConfigMapId]
		if !ok {
			return nil, false
		}
		return &Element{
			Kind:        KindConfigItem,
			Id:          i.Id,
			AppId:       parent.AppId,
			AppName:     appNames[parent.AppId],
			ParentId:    parent.Id,
			ParentName:  parent.SlugName,
			Name:        i.Key,
			Description: i.Description,
			UpdatedAt:   i.UpdatedAt,
		}, true
	}), nil
}

// Delete удаляет выбранные элементы одной транзакцией с общим batch_id: либо
// всё, либо ничего. Элемент, включённый после загрузки списка, отменяет всю
// операцию (errs.ElementActive); уже удалённый кем-то другим — пропускается.
// Активные потомки выключенного родителя удалению не мешают: в кластер они и
// так не попадают.
func (u *Usecase) Delete(ctx context.Context, refs []Ref) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if len(refs) == 0 {
		return errs.InvalidRequest
	}
	for _, ref := range refs {
		if ref.Id == "" || !slices.Contains(deleteOrder, ref.Kind) {
			return errs.InvalidRequest
		}
	}

	groups := lo.GroupBy(refs, func(r Ref) string { return r.Kind })

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		batchId := new(u.auditRec.NewBatchId())

		for _, kind := range deleteOrder {
			for _, ref := range groups[kind] {
				if err := u.deleteOne(ctx, ref, batchId); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (u *Usecase) deleteOne(ctx context.Context, ref Ref, batchId *string) error {
	switch ref.Kind {
	case KindApp:
		cur, found, err := u.appSvc.Get(ctx, ref.Id, false)
		if err != nil {
			return fmt.Errorf("appSvc.Get: %w", err)
		}
		if !found {
			return nil
		}
		if cur.Active {
			return errs.ElementActive
		}
		return u.cascadeSvc.DeleteApp(ctx, cur, batchId)
	case KindSecret:
		cur, found, err := u.secretSvc.Get(ctx, ref.Id, false)
		if err != nil {
			return fmt.Errorf("secretSvc.Get: %w", err)
		}
		if !found {
			return nil
		}
		if cur.Active {
			return errs.ElementActive
		}
		return u.cascadeSvc.DeleteSecret(ctx, cur, batchId)
	case KindItem:
		cur, found, err := u.itemSvc.Get(ctx, ref.Id, false)
		if err != nil {
			return fmt.Errorf("itemSvc.Get: %w", err)
		}
		if !found {
			return nil
		}
		if cur.Active {
			return errs.ElementActive
		}
		return u.cascadeSvc.DeleteItem(ctx, cur, batchId)
	case KindConfigMap:
		cur, found, err := u.configMapSvc.Get(ctx, ref.Id, false)
		if err != nil {
			return fmt.Errorf("configMapSvc.Get: %w", err)
		}
		if !found {
			return nil
		}
		if cur.Active {
			return errs.ElementActive
		}
		return u.cascadeSvc.DeleteConfigMap(ctx, cur, batchId)
	case KindConfigItem:
		cur, found, err := u.configItemSvc.Get(ctx, ref.Id, false)
		if err != nil {
			return fmt.Errorf("configItemSvc.Get: %w", err)
		}
		if !found {
			return nil
		}
		if cur.Active {
			return errs.ElementActive
		}
		return u.cascadeSvc.DeleteConfigItem(ctx, cur, batchId)
	}
	return errs.InvalidRequest
}
