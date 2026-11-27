package integration

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
)

// Фильтр app_id совпадает с тем, что реально разрешает сессия: в выборку по
// приложению попадают админы, пользователи с all_apps и те, у кого оно есть
// в app_ids. Пользователь с пустым app_ids без all_apps не видит ничего и в
// выборку не попадает (раньше пустой список значил «доступ ко всем»).
func TestUsrListByAppAccess(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	appA, _ := s.newApp(t, "access-a")
	appB, _ := s.newApp(t, "access-b")

	create := func(username string, isAdmin, allApps bool, appIds []string) {
		t.Helper()
		_, err := s.usrUsecase.Create(ctx, &usrModel.Edit{
			Name:     new(username),
			Username: new(username),
			Password: new("Access-Passw0rd!"),
			IsAdmin:  new(isAdmin),
			AllApps:  new(allApps),
			AppIds:   appIds,
		})
		require.NoError(t, err)
	}
	create("access-only-a", false, false, []string{appA})
	create("access-only-b", false, false, []string{appB})
	create("access-both", false, false, []string{appA, appB})
	create("access-all", false, true, nil)
	create("access-all-with-list", false, true, []string{appB})
	create("access-none", false, false, nil)
	create("access-none-empty", false, false, []string{})
	create("access-admin", true, false, []string{appB})

	list := func(appId string, isAdmin *bool) []string {
		t.Helper()
		items, _, err := s.usrUsecase.List(ctx, &usrModel.ListReq{
			Search:  new("access-"),
			AppId:   &appId,
			IsAdmin: isAdmin,
		})
		require.NoError(t, err)
		return lo.Map(items, func(v *usrModel.Main, _ int) string { return v.Username })
	}

	assert.ElementsMatch(t,
		[]string{"access-only-a", "access-both", "access-all", "access-all-with-list"},
		list(appA, new(false)),
	)
	assert.ElementsMatch(t,
		[]string{"access-only-b", "access-both", "access-all", "access-all-with-list"},
		list(appB, new(false)),
	)
	// админ имеет доступ всегда, даже к app не из своего app_ids
	assert.Contains(t, list(appA, nil), "access-admin")
	// пользователь без приложений не попадает ни в одну выборку
	for _, appId := range []string{appA, appB} {
		assert.NotContains(t, list(appId, nil), "access-none")
		assert.NotContains(t, list(appId, nil), "access-none-empty")
	}
}
