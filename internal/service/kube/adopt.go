package kube

import (
	"context"
	"sort"
	"strings"

	"github.com/samber/lo"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
)

// adoptionRefusal решает, можно ли усыновить живой объект кластера с тем же
// именем (Create → AlreadyExists), и возвращает причину отказа либо "".
//
// Sync заменяет содержимое объекта целиком, поэтому усыновление чужого
// объекта (без лейбла managed-by=kusec) не должно уничтожать его данные.
// Потерей считаются:
//   - ключ живого объекта, которого нет в записи kusec, — он исчез бы;
//   - ключ с непустым значением в кластере и пустым в kusec — был бы затёрт.
//
// Изменённые непустые значения и новые ключи потерей не считаются: источник
// истины — kusec. Объект, уже помеченный managed-by=kusec, проверке не
// подлежит — он и так наш (попадает сюда при sync по одному приложению, если
// аннотирован другим app). В причине — только имена ключей, без значений.
func adoptionRefusal(labels map[string]string, live, want map[string][]byte) string {
	if labels[managedByLabelKey] == managedByLabelValue {
		return ""
	}

	var missing, emptied []string
	for key, liveValue := range live {
		wantValue, ok := want[key]
		switch {
		case !ok:
			missing = append(missing, key)
		case len(liveValue) > 0 && len(wantValue) == 0:
			emptied = append(emptied, key)
		}
	}
	if len(missing) == 0 && len(emptied) == 0 {
		return ""
	}
	sort.Strings(missing)
	sort.Strings(emptied)

	var details []string
	if len(missing) > 0 {
		details = append(details, "keys missing in kusec: "+strings.Join(missing, ", "))
	}
	if len(emptied) > 0 {
		details = append(details, "keys with empty value in kusec: "+strings.Join(emptied, ", "))
	}

	return "refused: the live object is not managed by kusec and adopting it would lose its data (" +
		strings.Join(details, "; ") +
		"); import the values from the cluster first or delete the live object"
}

// ownershipRefusal решает, можно ли записать в живой объект под управлением
// kusec данные записи (appId, recordId), и возвращает причину отказа либо "".
//
// Имена объектов двух записей могут совпасть, если их app живут в одном
// namespace: секрет "dags-main" app "airflow" и секрет "main" app
// "airflow-dags" — оба kusec-airflow-dags-main. Без проверки sync одного app
// перезаписал бы объект другого (а потом удалил бы его как лишний).
// Можно писать:
//   - в объект без лейбла kusec (его проверяет adoptionRefusal);
//   - в объект своего app или своей записи (запись перенесли в другой app);
//   - в объект, чей app-владелец удалён из kusec, — он бесхозный.
//
// recordIdKey — аннотация id записи (secret-id или configmap-id).
func (s *Service) ownershipRefusal(ctx context.Context, labels, annotations map[string]string, recordIdKey, appId, recordId string) string {
	if labels[managedByLabelKey] != managedByLabelValue {
		return ""
	}
	ownerApp := annotations[appIdAnnotation]
	if ownerApp == "" || ownerApp == appId {
		return ""
	}
	if recordId != "" && annotations[recordIdKey] == recordId {
		return ""
	}

	apps, _, err := s.appSvc.List(ctx, &appModel.ListReq{Ids: []string{ownerApp}})
	if err != nil {
		return "refused: cannot check the owner app of the live object: " + err.Error()
	}
	if !lo.ContainsBy(apps, func(a *appModel.Main) bool { return a.Id == ownerApp }) {
		return ""
	}

	return "refused: the live object is managed by kusec for another app (" + ownerApp +
		"); names of two records collide — rename one of them"
}
