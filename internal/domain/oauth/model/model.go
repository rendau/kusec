package model

import (
	"time"

	commonModel "github.com/rendau/kusec/internal/domain/common/model"
)

const (
	TokenKindCode    = "code"
	TokenKindAccess  = "access"
	TokenKindRefresh = "refresh"
)

const (
	ClientAuthNone        = "none"
	ClientAuthSecretPost  = "client_secret_post"
	ClientAuthSecretBasic = "client_secret_basic"
)

// Client — OAuth-клиент из динамической регистрации. В БД не хранится:
// его данные несёт сам client_id, подписанный kusec (см. service.ClientCodec).
type Client struct {
	Name         string
	RedirectUris []string
	AuthMethod   string
}

// Grant — подключение OAuth-клиента, разрешённое пользователем на странице
// согласия. Его токены работают как mcp_only-ключ этого пользователя.
type Grant struct {
	Id            string
	UsrId         int64
	ClientId      string
	ClientName    string
	RedirectUri   string
	CodeChallenge string
	CreatedAt     time.Time
	ConnectedAt   *time.Time
	LastUsedAt    *time.Time
}

type GrantListReq struct {
	commonModel.ListParams

	UsrId     *int64
	Connected *bool
}

// Token — код авторизации, access- или refresh-токен подключения. Hash —
// sha256 токена; сам токен не хранится.
type Token struct {
	Hash      string
	GrantId   string
	Kind      string
	ExpiresAt time.Time
}
