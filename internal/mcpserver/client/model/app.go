package model

type App struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Active      bool   `json:"active"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	SlugName    string `json:"slug_name"`
	Description string `json:"description"`
}

// List

type AppListReq struct {
	ListParams ListParams
	Active     *bool
	Namespace  *string
	Search     *string
}

type AppListRep struct {
	PaginationInfo PaginationInfo `json:"pagination_info"`
	Results        []App          `json:"results"`
}

// Create

type AppCreateReq struct {
	Active      *bool  `json:"active,omitempty"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	SlugName    string `json:"slug_name"`
	Description string `json:"description"`
}

type AppCreateRep struct {
	ID string `json:"id"`
}

// Update

type AppUpdateReq struct {
	Active      *bool   `json:"active,omitempty"`
	Namespace   *string `json:"namespace,omitempty"`
	Name        *string `json:"name,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
}
