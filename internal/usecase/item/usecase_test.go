package item

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	"github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
)

// ── Моки ────────────────────────────────────────────────

// itemSvcMock запоминает параметры List: до репозитория (и поиска в памяти)
// запрос с недопустимой строкой поиска дойти не должен.
type itemSvcMock struct {
	listCalls []*model.ListReq
}

func (m *itemSvcMock) List(_ context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	m.listCalls = append(m.listCalls, pars)
	return []*model.Main{}, 0, nil
}

func (m *itemSvcMock) Get(context.Context, string, bool) (*model.Main, bool, error) {
	return nil, false, errs.NotImplemented
}

func (m *itemSvcMock) Create(context.Context, *model.Edit) (string, error) {
	return "", errs.NotImplemented
}

func (m *itemSvcMock) Update(context.Context, string, *model.Edit) error { return errs.NotImplemented }
func (m *itemSvcMock) Delete(context.Context, string) error              { return errs.NotImplemented }

type secretSvcMock struct{}

func (secretSvcMock) Get(_ context.Context, id string, _ bool) (*secretModel.Main, bool, error) {
	return &secretModel.Main{Id: id, AppId: "app1"}, true, nil
}

func (secretSvcMock) List(_ context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
	result := make([]*secretModel.Main, 0, len(pars.Ids))
	for _, id := range pars.Ids {
		result = append(result, &secretModel.Main{Id: id, AppId: "app1"})
	}
	return result, int64(len(result)), nil
}

// Строка поиска item-ов ограничена по длине — для любого вида List-запроса и
// любой сессии. При включённом шифровании поиск по значению идёт в памяти, и
// длинный шаблон с % и _ против большого значения занимал бы ядро на десятки
// секунд; отказ выдаётся до обращения к данным.
func TestList_SearchLengthLimited(t *testing.T) {
	t.Parallel()

	sessionSvc := sessionService.New("test-secret")

	sessions := map[string]*sessionModel.Session{
		"админ": {Id: 1, Admin: true},
		"не-админ с приложением": {Id: 2, AppIds: []string{"app1"}},
		"read_only-ключ":         {Id: 1, Admin: true, Scope: "read_only", Source: "api", ApiKeyId: "k"},
		"mcp-ключ":               {Id: 2, AppIds: []string{"app1"}, Scope: "mcp_only", Source: "mcp", ApiKeyId: "k"},
	}
	// все виды List-запроса: по секрету, по набору секретов, по приложению
	scopes := map[string]func() *model.ListReq{
		"по секрету":     func() *model.ListReq { return &model.ListReq{SecretId: new("sec1")} },
		"по секретам":    func() *model.ListReq { return &model.ListReq{SecretIds: []string{"sec1", "sec2"}} },
		"по приложению":  func() *model.ListReq { return &model.ListReq{AppId: new("app1")} },
		"с пагинацией":   func() *model.ListReq { return &model.ListReq{SecretId: new("sec1"), ListParams: pageOf(50)} },
		"только подсчёт": func() *model.ListReq { return &model.ListReq{AppId: new("app1"), ListParams: countOnly()} },
	}

	tooLong := map[string]string{
		"на символ длиннее":       strings.Repeat("a", searchMaxLen+1),
		"шаблон из % и _":         strings.Repeat("%_", searchMaxLen),
		"не-ASCII, длина в рунах": strings.Repeat("я", searchMaxLen+1),
	}

	for sessionName, session := range sessions {
		for scopeName, newPars := range scopes {
			svc := &itemSvcMock{}
			usecase := New(svc, secretSvcMock{}, sessionSvc, nil, nil)
			ctx := sessionSvc.WithContext(context.Background(), session)

			for searchName, search := range tooLong {
				pars := newPars()
				pars.Search = &search

				_, _, err := usecase.List(ctx, pars)

				fullErr, ok := errors.AsType[errs.ErrFull](err)
				require.True(t, ok, "%s / %s / %s: expected errs.ErrFull, got: %v", sessionName, scopeName, searchName, err)
				assert.Equal(t, errs.InvalidRequest, fullErr.Err)
				assert.Equal(t, "search is too long: at most 200 characters", fullErr.Desc)
			}
			assert.Empty(t, svc.listCalls, "%s / %s: до данных запрос дойти не должен", sessionName, scopeName)

			// ровно на пределе — допустимо (длина считается в символах, не в байтах)
			for _, search := range []string{strings.Repeat("a", searchMaxLen), strings.Repeat("я", searchMaxLen), "db_pass"} {
				pars := newPars()
				pars.Search = &search

				_, _, err := usecase.List(ctx, pars)
				require.NoError(t, err, "%s / %s", sessionName, scopeName)
			}
			assert.Len(t, svc.listCalls, 3, "%s / %s", sessionName, scopeName)
		}
	}

	// широкий запрос админа (без секрета и приложения) — тот же предел
	svc := &itemSvcMock{}
	usecase := New(svc, secretSvcMock{}, sessionSvc, nil, nil)
	_, _, err := usecase.List(
		sessionSvc.WithContext(context.Background(), &sessionModel.Session{Id: 1, Admin: true}),
		&model.ListReq{ListParams: pageOf(50), Search: new(strings.Repeat("a", searchMaxLen+1))},
	)
	fullErr, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "expected errs.ErrFull, got: %v", err)
	assert.Equal(t, errs.InvalidRequest, fullErr.Err)
	assert.Empty(t, svc.listCalls)
}

func pageOf(size int64) commonModel.ListParams {
	return commonModel.ListParams{PageSize: size, WithTotalCount: true}
}

func countOnly() commonModel.ListParams {
	return commonModel.ListParams{OnlyCount: true, WithTotalCount: true}
}
