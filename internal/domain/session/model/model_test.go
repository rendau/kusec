package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Доступ к приложениям: админ и all_apps — ко всем; иначе только к
// перечисленным, а пустой список — ни к одному (раньше он значил «ко всем»).
func TestAccessibleAppIds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		session *Session
		wantIds []string
		wantAll bool
		// доступ к приложениям app1 и other
		wantApp1, wantOther bool
	}{
		{
			name:    "nil-сессия — ничего",
			session: nil,
		},
		{
			name:    "неавторизованная сессия — ничего",
			session: &Session{},
		},
		{
			name:    "неавторизованная сессия с флагами — ничего",
			session: &Session{Admin: true, AllApps: true, AppIds: []string{"app1"}},
		},
		{
			name:    "админ — всё",
			session: &Session{Id: 1, Admin: true},
			wantAll: true, wantApp1: true, wantOther: true,
		},
		{
			name:    "админ со списком — всё равно всё",
			session: &Session{Id: 1, Admin: true, AppIds: []string{"app1"}},
			wantAll: true, wantApp1: true, wantOther: true,
		},
		{
			name:    "не-админ с all_apps — всё",
			session: &Session{Id: 2, AllApps: true},
			wantAll: true, wantApp1: true, wantOther: true,
		},
		{
			name:    "не-админ с all_apps и списком — всё",
			session: &Session{Id: 2, AllApps: true, AppIds: []string{"app1"}},
			wantAll: true, wantApp1: true, wantOther: true,
		},
		{
			name:     "не-админ со списком — только список",
			session:  &Session{Id: 2, AppIds: []string{"app1"}},
			wantIds:  []string{"app1"},
			wantApp1: true,
		},
		{
			name:    "не-админ с пустым списком — ничего",
			session: &Session{Id: 2, AppIds: []string{}},
		},
		{
			name:    "не-админ без списка — ничего",
			session: &Session{Id: 2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ids, all := tc.session.AccessibleAppIds()
			assert.Equal(t, tc.wantAll, all)
			assert.ElementsMatch(t, tc.wantIds, ids)

			assert.Equal(t, tc.wantApp1, tc.session.HasAppAccess("app1"))
			assert.Equal(t, tc.wantOther, tc.session.HasAppAccess("other"))
			// пустой id приложения доступа не даёт никому, кроме «всех»
			assert.Equal(t, tc.wantAll, tc.session.HasAppAccess(""))
		})
	}
}
