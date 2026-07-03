package model

import (
	"fmt"
	"strconv"
	"strings"
)

// Int64Str — int64, который protojson сериализует строкой ("123").
// Принимает при разборе и число, и строку.
type Int64Str int64

func (v *Int64Str) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*v = 0
		return nil
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("int64 from %q: %w", s, err)
	}

	*v = Int64Str(n)

	return nil
}

// ListParams — общие параметры list-запросов (query `list_params.*`).
// Пагинация zero-based: первая страница — Page=0.
type ListParams struct {
	Page           int64
	PageSize       int64
	WithTotalCount bool
}

type PaginationInfo struct {
	Page       Int64Str `json:"page"`
	PageSize   Int64Str `json:"page_size"`
	TotalCount Int64Str `json:"total_count"`
}

// ErrorRep — тело семантической ошибки gateway (common.ErrorRep).
type ErrorRep struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields"`
}
