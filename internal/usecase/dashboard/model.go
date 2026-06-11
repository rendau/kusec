package dashboard

import "time"

// Count — счётчик сущности: всего и из них активных.
type Count struct {
	Total  int64
	Active int64
}

// RecentSecret — секрет из блока «последние обновлённые», обогащённый
// именем приложения и количеством items.
type RecentSecret struct {
	Id          string
	AppId       string
	AppName     string
	SlugName    string
	Description string
	Active      bool
	UpdatedAt   time.Time
	ItemCount   int64
}

// Summary — данные дашборда одной структурой.
type Summary struct {
	App           Count
	Secret        Count
	Item          Count
	Usr           Count
	RecentSecrets []*RecentSecret
}
