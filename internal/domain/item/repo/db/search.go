package db

import (
	"context"
	"fmt"
	"strings"

	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	"github.com/rendau/kusec/internal/domain/item/model"
	repoModel "github.com/rendau/kusec/internal/domain/item/repo/db/model"
)

// searchPattern — шаблон LIKE «содержит подстроку». Один и тот же шаблон
// уходит и в SQL (ILIKE), и в поиск в памяти — результаты поиска не зависят
// от того, включено ли шифрование.
func searchPattern(search string) string {
	return "%" + search + "%"
}

// searchValueInMemory — нужно ли искать в памяти: шифрование включено и
// запрошен поиск по значению. Пустая строка поиска подходит любой строке —
// её оставляем SQL, чтобы не расшифровывать таблицу зря.
func (r *Repo) searchValueInMemory(pars *model.ListReq) bool {
	return r.box.Enabled() && pars != nil && pars.SearchValue && pars.Search != nil && *pars.Search != ""
}

// listSearchByValue — List с поиском по ключу, значению и описанию, когда
// значения в базе зашифрованы: SQL отбирает строки по остальным условиям,
// а поиск идёт здесь, после расшифровки. Строки читаются потоком, в памяти
// остаётся только запрошенная страница; total_count — число совпавших строк.
//
// Цена — чтение и расшифровка всех строк, подходящих под остальные условия
// (для запроса без ограничения по секрету/приложению — всей таблицы).
func (r *Repo) listSearchByValue(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	conditions, conditionExps := r.getConditions(pars, false)
	matcher := newLikeMatcher(searchPattern(*pars.Search))

	colNames := lo.Keys((&repoModel.Select{}).ListColumnMap())

	queryBuilder := r.QB.Select(colNames...).From(r.ModelStore.TableName).Where(conditions)
	for expression, args := range conditionExps {
		queryBuilder = queryBuilder.Where(expression, args...)
	}
	// для подсчёта порядок строк не важен — сортировку базе не заказываем
	if !pars.OnlyCount {
		sort := moboneTools.ConstructSortColumns(allowedSortFields, pars.Sort)
		if sort == nil {
			sort = (&repoModel.Select{}).DefaultSortColumns()
		}
		// id — последним в сортировке: страницы режутся в памяти, и порядок строк
		// с одинаковым ключом сортировки обязан совпадать от запроса к запросу
		queryBuilder = queryBuilder.OrderBy(append(sort, "id")...)
	}
	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("fail to build query: %w", err)
	}

	rows, err := r.ModelStore.GetConnection(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("fail to query: %w", err)
	}
	defer rows.Close()

	withCount := pars.WithTotalCount || pars.OnlyCount
	paged := pars.PageSize > 0
	pageFrom := pars.Page * pars.PageSize
	pageTo := pageFrom + pars.PageSize

	var matched int64
	items := make([]*repoModel.Select, 0)

	// строка читается в один и тот же item, пока он не попал в результат:
	// несовпавшие строки (их большинство) не оставляют после себя мусора
	item, scanDest := newScanItem(colNames)

	for rows.Next() {
		if err = rows.Scan(scanDest...); err != nil {
			return nil, 0, fmt.Errorf("fail to scan: %w", err)
		}
		if err = r.openValue(item); err != nil {
			return nil, 0, err
		}
		if !matcher.Match(item.Key) && !matcher.Match(item.Value) && !matcher.Match(item.Description) {
			continue
		}

		pos := matched
		matched++

		if pars.OnlyCount {
			continue
		}
		if paged && pos >= pageTo {
			if !withCount {
				break // страница набрана, общее число не просили — дальше не читаем
			}
			continue
		}
		if paged && pos < pageFrom {
			continue
		}
		items = append(items, item)
		item, scanDest = newScanItem(colNames)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows.Err: %w", err)
	}

	if !withCount {
		matched = 0 // как в mobone: без WithTotalCount общее число не отдаётся
	}
	return lo.Map(items, repoModel.EncodeSelect), matched, nil
}

// newScanItem — пустая строка выборки и указатели на её поля в порядке colNames.
func newScanItem(colNames []string) (*repoModel.Select, []any) {
	item := &repoModel.Select{}
	colMap := item.ListColumnMap()
	return item, lo.Map(colNames, func(colName string, _ int) any { return colMap[colName] })
}

// likeMatcher проверяет строку по шаблону LIKE без учёта регистра — как ILIKE
// в Postgres: % — любая последовательность символов (в том числе пустая),
// _ — ровно один любой символ, \ — следующий символ шаблона берётся буквально.
type likeMatcher struct {
	tokens []likeToken

	// substr — быстрый путь для обычного поиска: шаблон вида %текст% без
	// спецсимволов внутри сводится к поиску подстроки.
	substr   string
	isSubstr bool
}

type likeTokenKind int8

const (
	likeLiteral likeTokenKind = iota // ровно этот символ
	likeAnyOne                       // _
	likeAnySeq                       // %
)

type likeToken struct {
	kind likeTokenKind
	char rune
}

func newLikeMatcher(pattern string) *likeMatcher {
	pattern = strings.ToLower(pattern)

	m := &likeMatcher{}
	escaped := false
	for _, char := range pattern {
		switch {
		case escaped:
			m.tokens = append(m.tokens, likeToken{kind: likeLiteral, char: char})
			escaped = false
		case char == '\\':
			escaped = true
		case char == '%':
			m.tokens = append(m.tokens, likeToken{kind: likeAnySeq})
		case char == '_':
			m.tokens = append(m.tokens, likeToken{kind: likeAnyOne})
		default:
			m.tokens = append(m.tokens, likeToken{kind: likeLiteral, char: char})
		}
	}

	// %текст% из одних обычных символов — поиск подстроки
	if n := len(m.tokens); n >= 2 && m.tokens[0].kind == likeAnySeq && m.tokens[n-1].kind == likeAnySeq {
		inner := m.tokens[1 : n-1]
		if lo.EveryBy(inner, func(t likeToken) bool { return t.kind == likeLiteral }) {
			m.substr = string(lo.Map(inner, func(t likeToken, _ int) rune { return t.char }))
			m.isSubstr = true
		}
	}

	return m
}

func (m *likeMatcher) Match(s string) bool {
	s = strings.ToLower(s)
	if m.isSubstr {
		return strings.Contains(s, m.substr)
	}

	chars := []rune(s)
	ti, ci := 0, 0
	// возврат к последнему %: с какого токена и с какого символа пробовать снова
	backToken, backChar := -1, 0

	for ci < len(chars) {
		switch {
		case ti < len(m.tokens) && m.tokens[ti].kind == likeAnySeq:
			backToken, backChar = ti, ci
			ti++
		case ti < len(m.tokens) && (m.tokens[ti].kind == likeAnyOne || m.tokens[ti].char == chars[ci]):
			ti++
			ci++
		case backToken >= 0:
			// не сошлось: пусть последний % «съест» ещё один символ
			backChar++
			ti, ci = backToken+1, backChar
		default:
			return false
		}
	}
	// строка кончилась: остаток шаблона может состоять только из %
	for ti < len(m.tokens) && m.tokens[ti].kind == likeAnySeq {
		ti++
	}
	return ti == len(m.tokens)
}
