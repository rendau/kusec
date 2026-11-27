package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	configitemUsc "github.com/rendau/kusec/internal/usecase/configitem"
	itemUsc "github.com/rendau/kusec/internal/usecase/item"
)

// ── Часы и хранилища для тестов ─────────────────────────

// fakeClock — управляемое время для ограничителя.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// itemStoreMock — item-ы секретов в памяти с рабочими Create и Update.
type itemStoreMock struct {
	mu    sync.Mutex
	items map[string]*itemModel.Main
	seq   int
}

func (m *itemStoreMock) List(_ context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := lo.Filter(lo.Values(m.items), func(item *itemModel.Main, _ int) bool {
		return pars.SecretId == nil || item.SecretId == *pars.SecretId
	})
	return result, int64(len(result)), nil
}

func (m *itemStoreMock) Get(_ context.Context, id string, errNE bool) (*itemModel.Main, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[id]
	if !ok {
		if errNE {
			return nil, false, errs.ObjectNotFound
		}
		return nil, false, nil
	}
	return new(*item), true, nil
}

func (m *itemStoreMock) Create(_ context.Context, obj *itemModel.Edit) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.seq++
	id := fmt.Sprintf("new-%d", m.seq)
	m.items[id] = &itemModel.Main{
		Id:       id,
		SecretId: lo.FromPtr(obj.SecretId),
		Active:   true,
		Key:      lo.FromPtr(obj.Key),
		Value:    lo.FromPtr(obj.Value),
	}
	return id, nil
}

func (m *itemStoreMock) Update(_ context.Context, id string, obj *itemModel.Edit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[id]
	if !ok {
		return errs.ObjectNotFound
	}
	if obj.Value != nil {
		item.Value = *obj.Value
	}
	if obj.Description != nil {
		item.Description = *obj.Description
	}
	return nil
}

func (m *itemStoreMock) Delete(context.Context, string) error { return nil }

func (m *itemStoreMock) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

func (m *itemStoreMock) value(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[id].Value
}

// configItemStoreMock — item-ы конфигмапов: только создание.
type configItemStoreMock struct {
	mu    sync.Mutex
	items map[string]*configitemModel.Main
}

func (m *configItemStoreMock) List(context.Context, *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
	return nil, 0, nil
}

func (m *configItemStoreMock) Get(_ context.Context, id string, _ bool) (*configitemModel.Main, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[id]
	if !ok {
		return nil, false, errs.ObjectNotFound
	}
	return item, true, nil
}

func (m *configItemStoreMock) Create(_ context.Context, obj *configitemModel.Edit) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("ci-%d", len(m.items)+1)
	m.items[id] = &configitemModel.Main{
		Id:          id,
		ConfigMapId: lo.FromPtr(obj.ConfigMapId),
		Active:      true,
		Key:         lo.FromPtr(obj.Key),
		Value:       lo.FromPtr(obj.Value),
	}
	return id, nil
}

func (m *configItemStoreMock) Update(context.Context, string, *configitemModel.Edit) error {
	return nil
}
func (m *configItemStoreMock) Delete(context.Context, string) error { return nil }

func (m *configItemStoreMock) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

type configMapSvcStub struct{}

func (configMapSvcStub) Get(_ context.Context, id string, _ bool) (*configmapModel.Main, bool, error) {
	return &configmapModel.Main{Id: id, AppId: "app1", Active: true, SlugName: "main"}, true, nil
}

func (configMapSvcStub) List(context.Context, *configmapModel.ListReq) ([]*configmapModel.Main, int64, error) {
	return nil, 0, nil
}

// ── Ограничитель ────────────────────────────────────────

func TestWriteLimiter_BurstAndRefill(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	limiter := newWriteLimiter(20, 6*time.Second)
	limiter.now = clock.Now

	// запас: 20 записей подряд
	for i := range 20 {
		wait, ok := limiter.take("k1")
		require.True(t, ok, "запись %d", i+1)
		assert.Zero(t, wait)
	}

	// 21-я — отказ, ждать до следующего пополнения
	wait, ok := limiter.take("k1")
	assert.False(t, ok)
	assert.Equal(t, 6*time.Second, wait)

	// отказ ничего не списывает: срок ожидания не растёт
	clock.Advance(2 * time.Second)
	wait, ok = limiter.take("k1")
	assert.False(t, ok)
	assert.Equal(t, 4*time.Second, wait)

	// через 6 секунд от исчерпания — ровно одна запись
	clock.Advance(4 * time.Second)
	_, ok = limiter.take("k1")
	assert.True(t, ok)
	wait, ok = limiter.take("k1")
	assert.False(t, ok)
	assert.Equal(t, 6*time.Second, wait)

	// за минуту набегает 10 записей, не больше
	clock.Advance(time.Minute)
	for i := range 10 {
		_, ok = limiter.take("k1")
		require.True(t, ok, "запись %d после минуты", i+1)
	}
	_, ok = limiter.take("k1")
	assert.False(t, ok)

	// после долгого простоя запас полный, но не больше 20
	clock.Advance(24 * time.Hour)
	for i := range 20 {
		_, ok = limiter.take("k1")
		require.True(t, ok, "запись %d после простоя", i+1)
	}
	_, ok = limiter.take("k1")
	assert.False(t, ok)
}

func TestWriteLimiter_PerPrincipal(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	limiter := newWriteLimiter(2, time.Second)
	limiter.now = clock.Now

	for range 2 {
		_, ok := limiter.take("k1")
		require.True(t, ok)
	}
	_, ok := limiter.take("k1")
	assert.False(t, ok)

	// у другого подключения свой запас
	_, ok = limiter.take("k2")
	assert.True(t, ok)
}

// Подключения с восстановившимся запасом забываются — карта не растёт.
func TestWriteLimiter_Sweep(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	limiter := newWriteLimiter(2, time.Second)
	limiter.now = clock.Now

	for i := range 100 {
		_, ok := limiter.take(fmt.Sprintf("k%d", i))
		require.True(t, ok)
	}
	assert.Len(t, limiter.fullAt, 100)

	clock.Advance(time.Minute)
	_, ok := limiter.take("fresh")
	require.True(t, ok)
	assert.Len(t, limiter.fullAt, 1)
}

func TestWriteLimiter_Concurrent(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	limiter := newWriteLimiter(20, 6*time.Second)
	limiter.now = clock.Now

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
	)
	for range 100 {
		wg.Go(func() {
			if _, ok := limiter.take("k1"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	assert.Equal(t, 20, allowed)
}

func TestWritesKnownValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  ValueSourceIn
		want bool
	}{
		{name: "literal", src: ValueSourceIn{Kind: "literal", Value: "x"}, want: true},
		{name: "template с literal в vars", src: ValueSourceIn{
			Kind: "template", Template: "{{a}}", Vars: map[string]TemplateVarIn{"a": {Kind: "literal", Value: "x"}},
		}, want: true},
		// текст шаблона тоже пишет агент: «догадка{{пустой item}}»
		{name: "template с copy_item", src: ValueSourceIn{
			Kind: "template", Template: "guess{{a}}", Vars: map[string]TemplateVarIn{"a": {Kind: "copy_item", ItemId: "i1"}},
		}, want: true},
		{name: "template из реестра", src: ValueSourceIn{Kind: "template", Template: "{{a}}{{b}}"}, want: true},
		{name: "generate", src: ValueSourceIn{Kind: "generate"}, want: false},
		{name: "reuse", src: ValueSourceIn{Kind: "reuse", Name: "x"}, want: false},
		{name: "copy_item", src: ValueSourceIn{Kind: "copy_item", ItemId: "i1"}, want: false},
		{name: "неизвестный kind", src: ValueSourceIn{Kind: "nope"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, writesKnownValue(tt.src))
		})
	}
}

// ── Сквозная проверка ───────────────────────────────────

type limitEnv struct {
	h           *Handler
	clock       *fakeClock
	items       *itemStoreMock
	configItems *configItemStoreMock
	key1        string
	key2        string
}

// newLimitEnv — MCP-хендлер с реальными usecase-ами item/config_item поверх
// хранилищ в памяти. key1 и key2 — два ключа одного пользователя (два разных
// principal). В секрете sec1 лежит item с неизвестным агенту значением
// (target) и пустой item (empty) — такие заводят под ручное заполнение.
func newLimitEnv(t *testing.T) *limitEnv {
	t.Helper()

	key1, hash1, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	key2, hash2, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	sessionSvc := sessionService.New("test-secret")
	apikeyUsecase := apikeyUsc.New(
		&apikeySvcMock{byHash: map[string]*apikeyModel.Main{
			hash1: {Id: "k1", UsrId: 10, Active: true, Scope: "mcp_only"},
			hash2: {Id: "k2", UsrId: 10, Active: true, Scope: "mcp_only"},
		}},
		&usrSvcMock{usrs: map[int64]*usrModel.Main{
			10: {Id: 10, Active: true, AppIds: []string{"app1"}},
		}},
		nil,
		txmStub{},
		auditRecStub{},
		nil,
	)

	items := &itemStoreMock{items: map[string]*itemModel.Main{
		"target": {Id: "target", SecretId: "sec1", Active: true, Key: "PIN", Value: "4829"},
		"empty":  {Id: "empty", SecretId: "sec1", Active: true, Key: "TG_TOKEN", Value: ""},
	}}
	configItems := &configItemStoreMock{items: map[string]*configitemModel.Main{}}

	itemUsecase := itemUsc.New(
		items,
		&secretSvcMock{secrets: map[string]*secretModel.Main{
			"sec1": {Id: "sec1", AppId: "app1", Active: true, SlugName: "db"},
		}},
		sessionSvc,
		txmStub{},
		auditRecStub{},
	)
	configitemUsecase := configitemUsc.New(configItems, configMapSvcStub{}, sessionSvc, txmStub{}, auditRecStub{})

	h := New(sessionSvc, apikeyUsecase, nil, nil, nil, itemUsecase, nil, configitemUsecase, nil, nil, nil)

	clock := newFakeClock()
	h.knownValueWrites.now = clock.Now

	return &limitEnv{
		h:           h,
		clock:       clock,
		items:       items,
		configItems: configItems,
		key1:        key1,
		key2:        key2,
	}
}

func (e *limitEnv) connect(t *testing.T, key string) *mcpsdk.ClientSession {
	t.Helper()

	httpSrv := httptest.NewServer(e.h.HTTPHandler())
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clSession.Close() })

	return clSession
}

// toolText — текст ответа инструмента (для ошибки — её текст).
func toolText(res *mcpsdk.CallToolResult) string {
	texts := lo.FilterMap(res.Content, func(c mcpsdk.Content, _ int) (string, bool) {
		text, ok := c.(*mcpsdk.TextContent)
		if !ok {
			return "", false
		}
		return text.Text, true
	})
	return strings.Join(texts, "\n")
}

func literalItem(key, value string) map[string]any {
	return map[string]any{
		"secret_id":    "sec1",
		"key":          key,
		"value_source": map[string]any{"kind": "literal", "value": value},
	}
}

// Подбор значения сравнением отпечатков: агент пишет в секрет свою догадку и
// сравнивает value_hash с чужим. Запас — 20 записей на подключение, дальше
// одна запись в 6 секунд; обойти ограничение шаблоном, второй сессией или
// update вместо create нельзя.
func TestKnownValueWriteLimitE2E(t *testing.T) {
	t.Parallel()

	env := newLimitEnv(t)
	agent := env.connect(t, env.key1)

	assertLimited := func(t *testing.T, res *mcpsdk.CallToolResult, raw string) {
		t.Helper()
		require.True(t, res.IsError, raw)
		text := toolText(res)
		assert.Contains(t, text, "повтори через 6 с")
		assert.Contains(t, text, "literal предназначен для несекретных значений")
	}

	// запас: 20 записей literal проходят, отпечаток возвращается
	for i := range knownValueWriteBurst {
		res, raw := call(t, agent, "create_item", literalItem(fmt.Sprintf("GUESS_%d", i), fmt.Sprintf("%04d", i)))
		require.False(t, res.IsError, raw)
		assert.Contains(t, raw, maskValue(fmt.Sprintf("%04d", i)).Hash)
	}
	created := env.items.count()

	// 21-я — отказ с объяснением; в базу ничего не записано
	res, raw := call(t, agent, "create_item", literalItem("GUESS_NEXT", "4829"))
	assertLimited(t, res, raw)
	assert.NotContains(t, raw, maskValue("4829").Hash, "отпечаток догадки не выдаётся")
	assert.Equal(t, created, env.items.count())

	t.Run("шаблон — не обход", func(t *testing.T) {
		for name, src := range map[string]map[string]any{
			// текст шаблона + пустой item: догадка без единого literal
			"текст шаблона и пустой item": {
				"kind": "template", "template": "4829{{e}}",
				"vars": map[string]any{"e": map[string]any{"kind": "copy_item", "item_id": "empty"}},
			},
			// склейка известных частей
			"склейка copy_item": {
				"kind": "template", "template": "{{a}}{{b}}",
				"vars": map[string]any{
					"a": map[string]any{"kind": "copy_item", "item_id": "new-1"},
					"b": map[string]any{"kind": "copy_item", "item_id": "new-2"},
				},
			},
			"literal в vars": {
				"kind": "template", "template": "{{a}}",
				"vars": map[string]any{"a": map[string]any{"kind": "literal", "value": "4829"}},
			},
		} {
			res, raw := call(t, agent, "create_item", map[string]any{
				"secret_id": "sec1", "key": "TPL", "value_source": src,
			})
			assertLimited(t, res, raw)
			assert.Equal(t, created, env.items.count(), name)
		}
	})

	t.Run("update — не обход", func(t *testing.T) {
		res, raw := call(t, agent, "update_item", map[string]any{
			"id": "new-1", "value_source": map[string]any{"kind": "literal", "value": "4829"},
		})
		assertLimited(t, res, raw)
		assert.Equal(t, "0000", env.items.value("new-1"), "значение не изменилось")
	})

	t.Run("вторая сессия тем же ключом — не обход", func(t *testing.T) {
		second := env.connect(t, env.key1)
		res, raw := call(t, second, "create_item", literalItem("GUESS_2ND", "4829"))
		assertLimited(t, res, raw)
	})

	t.Run("не ограничены: generate, copy_item, reuse, update без значения", func(t *testing.T) {
		// по порядку: reuse берёт значение, сгенерированное первым вызовом
		for _, src := range []map[string]any{
			{"kind": "generate", "name": "tok"},
			{"kind": "copy_item", "item_id": "target"},
			{"kind": "reuse", "name": "tok"},
		} {
			kind, _ := src["kind"].(string)
			res, raw := call(t, agent, "create_item", map[string]any{
				"secret_id": "sec1", "key": "OK_" + kind, "value_source": src,
			})
			require.False(t, res.IsError, "%s: %s", kind, raw)
		}

		res, raw := call(t, agent, "update_item", map[string]any{"id": "new-1", "description": "note"})
		require.False(t, res.IsError, raw)
	})

	t.Run("конфигмапы не ограничены", func(t *testing.T) {
		for i := range knownValueWriteBurst + 5 {
			res, raw := call(t, agent, "create_config_item", map[string]any{
				"configmap_id": "cm1", "key": fmt.Sprintf("HOST_%d", i), "value": "pg.internal",
			})
			require.False(t, res.IsError, raw)
		}
		assert.Equal(t, knownValueWriteBurst+5, env.configItems.count())
	})

	t.Run("у другого ключа свой запас", func(t *testing.T) {
		other := env.connect(t, env.key2)
		res, raw := call(t, other, "create_item", literalItem("OTHER", "pg.internal"))
		require.False(t, res.IsError, raw)
	})

	// через 6 секунд — ровно одна запись
	env.clock.Advance(knownValueWriteRefill)
	res, raw = call(t, agent, "create_item", literalItem("GUESS_LATER", "4830"))
	require.False(t, res.IsError, raw)
	res, raw = call(t, agent, "create_item", literalItem("GUESS_LATER_2", "4831"))
	assertLimited(t, res, raw)
}
