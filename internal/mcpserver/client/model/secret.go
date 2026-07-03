package model

type Secret struct {
	ID             string `json:"id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	AppID          string `json:"app_id"`
	Active         bool   `json:"active"`
	SlugName       string `json:"slug_name"`
	Description    string `json:"description"`
	KubeSecretName string `json:"kube_secret_name"`
	KubeType       string `json:"kube_type"`
	ExactSlug      bool   `json:"exact_slug"`
}

// List

type SecretListReq struct {
	ListParams ListParams
	AppID      *string
	Active     *bool
	Search     *string
}

type SecretListRep struct {
	PaginationInfo PaginationInfo `json:"pagination_info"`
	Results        []Secret       `json:"results"`
}

// Create

type SecretCreateReq struct {
	AppID       string `json:"app_id"`
	Active      *bool  `json:"active,omitempty"`
	SlugName    string `json:"slug_name"`
	Description string `json:"description"`
	KubeType    string `json:"kube_type,omitempty"`
}

type SecretCreateRep struct {
	ID string `json:"id"`
}

// Update

type SecretUpdateReq struct {
	Active      *bool   `json:"active,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
	KubeType    *string `json:"kube_type,omitempty"`
}
