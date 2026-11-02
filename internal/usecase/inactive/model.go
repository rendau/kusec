package inactive

import "time"

// Виды элементов; значения совпадают с audit.entity_type.
const (
	KindApp        = "app"
	KindSecret     = "secret"
	KindItem       = "item"
	KindConfigMap  = "configmap"
	KindConfigItem = "config_item"
)

// Element — деактивированный элемент в общем списке (без значений item-ов).
type Element struct {
	Kind        string
	Id          string
	AppId       string
	AppName     string
	ParentId    string
	ParentName  string
	Name        string
	Description string
	UpdatedAt   time.Time
	ChildCount  int64
}

// Ref — ссылка на элемент для удаления.
type Ref struct {
	Kind string
	Id   string
}
