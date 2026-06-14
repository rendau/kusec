package dashboard

import "time"

type Count struct {
	Total  int64
	Active int64
}

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

type Summary struct {
	App           Count
	Secret        Count
	Item          Count
	ConfigMap     Count
	ConfigItem    Count
	Usr           Count
	RecentSecrets []*RecentSecret
}
