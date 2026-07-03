package model

type ConfigItem struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	ConfigmapID string `json:"configmap_id"`
	Active      bool   `json:"active"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueFormat string `json:"value_format"`
	Encoding    string `json:"encoding"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Description string `json:"description"`
}

// List

type ConfigItemListReq struct {
	ListParams   ListParams
	ConfigmapID  *string
	ConfigmapIDs []string
	Active       *bool
	Search       *string
}

type ConfigItemListRep struct {
	PaginationInfo PaginationInfo `json:"pagination_info"`
	Results        []ConfigItem   `json:"results"`
}

// Create

type ConfigItemCreateReq struct {
	ConfigmapID string `json:"configmap_id"`
	Active      *bool  `json:"active,omitempty"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueFormat string `json:"value_format,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description"`
}

type ConfigItemCreateRep struct {
	ID string `json:"id"`
}

// Update

type ConfigItemUpdateReq struct {
	ConfigmapID *string `json:"configmap_id,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	Key         *string `json:"key,omitempty"`
	Value       *string `json:"value,omitempty"`
	ValueFormat *string `json:"value_format,omitempty"`
	Encoding    *string `json:"encoding,omitempty"`
	FileName    *string `json:"file_name,omitempty"`
	ContentType *string `json:"content_type,omitempty"`
	Description *string `json:"description,omitempty"`
}
