package mcp

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/errs"
)

func TestGenerateValue_Formats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		format  string
		length  int
		charset string
	}{
		{"", 32, alnumChars},
		{"alnum", 10, alnumChars},
		{"ascii", 64, alnumChars + asciiExtras},
		{"digits", 6, digitChars},
		{"hex", 33, "0123456789abcdef"},
		{"base64url", 43, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"},
	}

	for _, tc := range cases {
		t.Run("format_"+tc.format, func(t *testing.T) {
			t.Parallel()

			value, err := generateValue(tc.format, tc.length)
			require.NoError(t, err)
			require.Len(t, value, tc.length)

			for _, r := range value {
				assert.Contains(t, tc.charset, string(r))
			}
		})
	}
}

func TestGenerateValue_DefaultLength(t *testing.T) {
	t.Parallel()

	value, err := generateValue("", 0)
	require.NoError(t, err)
	assert.Len(t, value, defaultGenLength)
}

func TestGenerateValue_Uuid(t *testing.T) {
	t.Parallel()

	value, err := generateValue("uuid", 0)
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`), value)
}

func TestGenerateValue_Errors(t *testing.T) {
	t.Parallel()

	_, err := generateValue("unknown", 10)
	require.Error(t, err)

	_, err = generateValue("", maxGenLength+1)
	require.Error(t, err)
}

func TestGenerateValue_Unique(t *testing.T) {
	t.Parallel()

	a, err := generateValue("", 32)
	require.NoError(t, err)
	b, err := generateValue("", 32)
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
}

func TestMaskValue(t *testing.T) {
	t.Parallel()

	masked := maskValue("пароль")

	assert.Equal(t, 6, masked.Chars)
	assert.Equal(t, 12, masked.Bytes)
	assert.Len(t, masked.Hash, 16)

	// детерминированность и различимость
	assert.Equal(t, masked.Hash, maskValue("пароль").Hash)
	assert.NotEqual(t, masked.Hash, maskValue("другой").Hash)
}

func TestVault_RememberLookup(t *testing.T) {
	t.Parallel()

	v := newVault()
	v.remember("db_password", "secret-value-1")

	got, ok := v.lookup("db_password")
	require.True(t, ok)
	assert.Equal(t, "secret-value-1", got)

	_, ok = v.lookup("unknown")
	assert.False(t, ok)

	assert.Equal(t, []string{"db_password"}, v.names())
}

func TestResolveTemplate_FromVault(t *testing.T) {
	t.Parallel()

	s := &sessionServer{vault: newVault()}
	s.vault.remember("db_password", "pw-123")

	value, err := s.resolveValueSource(t.Context(), ValueSourceIn{
		Kind:     "template",
		Template: "postgres://app:{{ db_password }}@pg:5432/app",
	})
	require.NoError(t, err)
	assert.Equal(t, "postgres://app:pw-123@pg:5432/app", value)

	// итоговое значение помечено увиденным — вычищается из ошибок
	assert.Contains(t, s.vault.scrub("err: "+value), "[REDACTED]")
}

func TestResolveTemplate_Vars(t *testing.T) {
	t.Parallel()

	s := &sessionServer{vault: newVault()}

	value, err := s.resolveValueSource(t.Context(), ValueSourceIn{
		Kind:     "template",
		Name:     "mq_dsn",
		Template: "amqp://{{user}}:{{pass}}@mq:5672/",
		Vars: map[string]TemplateVarIn{
			"user": {Kind: "literal", Value: "app"},
			"pass": {Kind: "generate", Length: 16, Name: "mq_password"},
		},
	})
	require.NoError(t, err)

	// сгенерированная переменная и итог зарегистрированы в реестре сессии
	assert.Equal(t, []string{"mq_dsn", "mq_password"}, s.vault.names())

	pass, ok := s.vault.lookup("mq_password")
	require.True(t, ok)
	assert.Equal(t, "amqp://app:"+pass+"@mq:5672/", value)
}

func TestResolveTemplate_RepeatedVar(t *testing.T) {
	t.Parallel()

	s := &sessionServer{vault: newVault()}

	// один и тот же плейсхолдер с generate резолвится единожды
	value, err := s.resolveValueSource(t.Context(), ValueSourceIn{
		Kind:     "template",
		Template: "{{tok}}:{{tok}}",
		Vars:     map[string]TemplateVarIn{"tok": {Kind: "generate", Length: 8}},
	})
	require.NoError(t, err)

	parts := strings.Split(value, ":")
	require.Len(t, parts, 2)
	assert.Equal(t, parts[0], parts[1])
	assert.Len(t, parts[0], 8)
}

func TestResolveTemplate_Errors(t *testing.T) {
	t.Parallel()

	s := &sessionServer{vault: newVault()}

	_, err := s.resolveValueSource(t.Context(), ValueSourceIn{Kind: "template"})
	require.ErrorContains(t, err, "требуется template")

	_, err = s.resolveValueSource(t.Context(), ValueSourceIn{Kind: "template", Template: "без плейсхолдеров"})
	require.ErrorContains(t, err, "нет ни одного плейсхолдера")

	// в ошибке — номер плейсхолдера, а не имя: имя задаёт агент
	_, err = s.resolveValueSource(t.Context(), ValueSourceIn{Kind: "template", Template: "x-{{unknown}}"})
	require.ErrorContains(t, err, "плейсхолдер №1")
	assert.NotContains(t, err.Error(), "unknown")
}

// Подбор значения через текст ошибки: агент присылает список вариантов в поле,
// которое попадает в текст ошибки, и смотрит, какой из них вычищен. Ни ввод
// агента, ни место совпадения в ответ попадать не должны.
func TestToolErr_NoGuessingThroughErrorText(t *testing.T) {
	t.Parallel()

	s := &sessionServer{vault: newVault()}
	s.vault.markSeen("482913") // агент прочитал item: значение помечено увиденным

	candidates := "482912 482913 482914"

	// поля, которые раньше повторялись в тексте ошибки
	for _, src := range []ValueSourceIn{
		{Kind: candidates},
		{Kind: "generate", Format: candidates},
		{Kind: "reuse", Name: candidates},
		{Kind: "template", Template: "{{a}}", Vars: map[string]TemplateVarIn{"a": {Kind: candidates}}},
	} {
		_, err := s.resolveValueSource(t.Context(), src)
		require.Error(t, err)
		text := s.toolErr(err).Error()
		assert.NotContains(t, text, "482912", "ввод агента не повторяется в ошибке")
		assert.NotContains(t, text, "[REDACTED]")
	}

	// ошибка нижних слоёв (например, Postgres повторяет присланный id) — общий текст
	pgLike := fmt.Errorf("svc.Get: %w", errors.New(`invalid input syntax for type uuid: "`+candidates+`"`))
	assert.Equal(t, internalErrText, s.toolErr(pgLike).Error())

	// семантический код проходит как есть
	assert.Equal(t, "copy_item: object_not_found", s.toolErr(agentWrap("copy_item", fmt.Errorf("svc.Get: %w", errs.ObjectNotFound))).Error())

	// описание ErrFull с повтором ввода (ключ «уже есть») уходит как есть,
	// есть в нём увиденное значение или нет: подмена текста при совпадении
	// сама была бы ответом «угадал / не угадал» — агент клал бы в ключ тысячи
	// вариантов и делил их пополам
	hit := errs.ErrFull{Err: errs.ItemKeyExists, Desc: `item key "` + candidates + `" already exists in the secret`}
	miss := errs.ErrFull{Err: errs.ItemKeyExists, Desc: `item key "100001 100002" already exists in the secret`}
	assert.Equal(t, "item_key_exists: "+hit.Desc, s.toolErr(hit).Error())
	assert.Equal(t, "item_key_exists: "+miss.Desc, s.toolErr(miss).Error())
}

func TestVault_Scrub(t *testing.T) {
	t.Parallel()

	v := newVault()
	v.remember("token", "sUp3r-s3cr3t")
	v.markSeen("another-secret")
	v.markSeen("")

	scrubbed := v.scrub(`error: value "sUp3r-s3cr3t" conflicts with another-secret`)

	assert.NotContains(t, scrubbed, "sUp3r-s3cr3t")
	assert.NotContains(t, scrubbed, "another-secret")
	assert.True(t, strings.Contains(scrubbed, "[REDACTED]"))

	// текст без секретов не меняется
	assert.Equal(t, "обычная ошибка", v.scrub("обычная ошибка"))
}
