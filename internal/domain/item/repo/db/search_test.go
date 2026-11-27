package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Поиск в памяти обязан вести себя как `ILIKE '%' || search || '%'` в Postgres:
// результат поиска не должен зависеть от того, включено ли шифрование.
func TestLikeMatcherSearchPattern(t *testing.T) {
	cases := []struct {
		search string
		value  string
		want   bool
	}{
		// подстрока без учёта регистра
		{"secret", "My-Secret-Value", true},
		{"SECRET", "my-secret-value", true},
		{"secret", "secret", true},
		{"secret", "secre", false},
		{"secret", "", false},
		{"пароль", "Мой ПАРОЛЬ здесь", true},
		{"пароль", "password", false},
		// значение из нескольких строк
		{"line2", "line1\nline2\nline3", true},

		// _ — ровно один любой символ
		{"pg_pass", "PG_PASSWORD", true},
		{"pg_pass", "PG-PASSWORD", true},
		{"pg_pass", "PGPASSWORD", false},
		{"a_c", "a\nc", true},
		{"_", "", false},
		{"_", "x", true},
		{"п_р", "ПАРОЛЬ", true},

		// % — любая последовательность, в том числе пустая
		{"db%url", "DB_PRIMARY_URL", true},
		{"db%url", "dburl", true},
		{"db%url", "url-db", false},
		{"a%b%c", "xxaxxbxxcxx", true},
		{"a%b%c", "xxaxxcxxbxx", false},
		{"%", "", true},
		{"%", "anything", true},

		// возврат назад после неудачной попытки
		{"ab_d", "abxabcd", true},
		{"a%bc", "abxbc", true},
		{"aab", "aaab", true},

		// \ — следующий символ буквально
		{`100\%`, "скидка 100%", true},
		{`100\%`, "1000", false},
		{`a\_b`, "a_b", true},
		{`a\_b`, "axb", false},
		{`a\\b`, `a\b`, true},
		// строка поиска кончается на \: он экранирует завершающий % шаблона,
		// совпадение — только в конце строки
		{`foo\`, "foo%", true},
		{`foo\`, "xfoo%", true},
		{`foo\`, "foo%x", false},
		{`foo\`, "foo", false},
	}

	for _, c := range cases {
		got := newLikeMatcher(searchPattern(c.search)).Match(c.value)
		assert.Equal(t, c.want, got, "search=%q value=%q", c.search, c.value)
	}
}

// Обычный поиск (без спецсимволов LIKE) сводится к поиску подстроки.
func TestLikeMatcherSubstringFastPath(t *testing.T) {
	assert.True(t, newLikeMatcher(searchPattern("token")).isSubstr)
	assert.True(t, newLikeMatcher(searchPattern(`100\%`)).isSubstr)
	assert.False(t, newLikeMatcher(searchPattern("pg_pass")).isSubstr)
	assert.False(t, newLikeMatcher(searchPattern("a%b")).isSubstr)
	assert.False(t, newLikeMatcher(searchPattern(`foo\`)).isSubstr)
}
