package integration

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
)

// Фильтр app_id совпадает с тем, что реально разрешает сессия: пользователь с
// пустым app_ids видит все app, поэтому попадает в выборку по любому из них.
func TestUsrListByAppAccess(t *testing.T) {
	s := newStack(t)
	ctx := s.adminCtx()

	appA, _ := s.newApp(t, "access-a")
	appB, _ := s.newApp(t, "access-b")

	create := func(username string, isAdmin bool, appIds []string) {
		t.Helper()
		_, err := s.usrUsecase.Create(ctx, &usrModel.Edit{
			Name:     new(username),
			Username: new(username),
			Password: new("Access-Passw0rd!"),
			IsAdmin:  new(isAdmin),
			AppIds:   appIds,
		})
		require.NoError(t, err)
	}
	create("access-only-a", false, []string{appA})
	create("access-only-b", false, []string{appB})
	create("access-both", false, []string{appA, appB})
	create("access-all", false, nil)
	create("access-admin", true, []string{appB})

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
		[]string{"access-only-a", "access-both", "access-all"},
		list(appA, new(false)),
	)
	assert.ElementsMatch(t,
		[]string{"access-only-b", "access-both", "access-all"},
		list(appB, new(false)),
	)
	// админ имеет доступ всегда, даже к app не из своего app_ids
	assert.Contains(t, list(appA, nil), "access-admin")
}
