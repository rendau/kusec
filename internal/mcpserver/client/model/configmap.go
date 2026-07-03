package model

type ConfigMap struct {
	ID                string `json:"id"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	AppID             string `json:"app_id"`
	Active            bool   `json:"active"`
	SlugName          string `json:"slug_name"`
	Description       string `json:"description"`
	KubeConfigmapName string `json:"kube_configmap_name"`
	ExactSlug         bool   `json:"exact_slug"`
}

// List

type ConfigMapListReq struct {
	ListParams ListParams
	AppID      *string
	Active     *bool
	Search     *string
}

type ConfigMapListRep struct {
	PaginationInfo PaginationInfo `json:"pagination_info"`
	Results        []ConfigMap    `json:"results"`
}

// Create

type ConfigMapCreateReq struct {
	AppID       string `json:"app_id"`
	Active      *bool  `json:"active,omitempty"`
	SlugName    string `json:"slug_name"`
	Description string `json:"description"`
}

type ConfigMapCreateRep struct {
	ID string `json:"id"`
}

// Update

type ConfigMapUpdateReq struct {
	Active      *bool   `json:"active,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
}
