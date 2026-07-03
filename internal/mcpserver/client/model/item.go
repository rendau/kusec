package model

// Item — item секрета. Поле Value — чувствительное: за пределы клиента и
// value-механики MCP-сервера оно выходить не должно.
type Item struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	SecretID    string `json:"secret_id"`
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

type ItemListReq struct {
	ListParams ListParams
	SecretID   *string
	SecretIDs  []string
	Active     *bool
	Search     *string
}

type ItemListRep struct {
	PaginationInfo PaginationInfo `json:"pagination_info"`
	Results        []Item         `json:"results"`
}

// Create

type ItemCreateReq struct {
	SecretID    string `json:"secret_id"`
	Active      *bool  `json:"active,omitempty"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueFormat string `json:"value_format,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description"`
}

type ItemCreateRep struct {
	ID string `json:"id"`
}

// Update

type ItemUpdateReq struct {
	SecretID    *string `json:"secret_id,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	Key         *string `json:"key,omitempty"`
	Value       *string `json:"value,omitempty"`
	ValueFormat *string `json:"value_format,omitempty"`
	Encoding    *string `json:"encoding,omitempty"`
	FileName    *string `json:"file_name,omitempty"`
	ContentType *string `json:"content_type,omitempty"`
	Description *string `json:"description,omitempty"`
}
