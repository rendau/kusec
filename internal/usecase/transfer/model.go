package transfer

import "time"

// ── Import ──────────────────────────────────────────────

type ImportItem struct {
	Key         string
	Value       string
	ValueFormat string
	Encoding    string
	FileName    string
	ContentType string
	Description string
	Active      *bool
}

type ImportSecret struct {
	SlugName    string
	Description string
	Active      *bool
	Items       []*ImportItem
}

type ImportApp struct {
	Namespace   string
	Name        string
	SlugName    string
	Description string
	Active      *bool
	Secrets     []*ImportSecret
}

type ImportReq struct {
	Apps []*ImportApp
}

type ImportResult struct {
	AppsCreated    int64
	AppsUpdated    int64
	SecretsCreated int64
	SecretsUpdated int64
	ItemsCreated   int64
	ItemsUpdated   int64
	Unchanged      int64

	// Ошибки отдельных записей; импорт продолжается для остальных.
	Errors []string
}

// ── Tree (export без значений) ──────────────────────────

type TreeItem struct {
	Id          string
	Key         string
	ValueFormat string
	Encoding    string
	FileName    string
	ContentType string
	Description string
	Active      bool
	UpdatedAt   time.Time
	ValueSize   int64
}

type TreeSecret struct {
	Id          string
	SlugName    string
	Description string
	Active      bool
	UpdatedAt   time.Time
	Items       []*TreeItem
}

type TreeApp struct {
	Id          string
	Namespace   string
	Name        string
	SlugName    string
	Description string
	Active      bool
	UpdatedAt   time.Time
	Secrets     []*TreeSecret
}
