package model

// Auth

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TotpCode string `json:"totp_code,omitempty"`
}

type LoginRep struct {
	Jwt               string `json:"jwt"`
	RefreshToken      string `json:"refresh_token"`
	TotpRequired      bool   `json:"totp_required"`
	TotpSetupRequired bool   `json:"totp_setup_required"`
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token"`
}
