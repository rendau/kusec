package transfer

import "time"

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
	KubeType    string
	UpdatedAt   time.Time
	Items       []*TreeItem
}

type TreeConfigItem struct {
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

type TreeConfigMap struct {
	Id          string
	SlugName    string
	Description string
	Active      bool
	UpdatedAt   time.Time
	Items       []*TreeConfigItem
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
	ConfigMaps  []*TreeConfigMap
}
