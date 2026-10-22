package config

import (
	"time"

	"github.com/caarlos0/env/v9"
	_ "github.com/joho/godotenv/autoload"
)

var Conf = struct {
	Debug    bool   `env:"DEBUG" envDefault:"false"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	WithTracing   bool   `env:"WITH_TRACING" envDefault:"false"`
	JaegerAddress string `env:"JAEGER_ADDRESS"`

	GrpcPort string `env:"GRPC_PORT" envDefault:"5050"`
	HttpPort string `env:"HTTP_PORT" envDefault:"80"`

	// Встроенный MCP-сервер для AI-агентов (см. docs/mcp-server.md):
	// отдельный HTTP-порт, аутентификация по api-key.
	McpEnabled bool   `env:"MCP_ENABLED" envDefault:"false"`
	McpPort    string `env:"MCP_PORT" envDefault:"5060"`

	// OAuth-вход MCP-клиентов, подключающихся по адресу (коннекторы Claude);
	// api-ключи работают рядом. McpPublicUrl — адрес MCP-сервера, как его
	// видят клиенты; McpOAuthAdminUrl — адрес админки (страница согласия).
	McpOAuthEnabled      bool          `env:"MCP_OAUTH_ENABLED" envDefault:"false"`
	McpPublicUrl         string        `env:"MCP_PUBLIC_URL"`
	McpOAuthAdminUrl     string        `env:"MCP_OAUTH_ADMIN_URL"`
	McpOAuthRedirectUris []string      `env:"MCP_OAUTH_REDIRECT_URIS" envSeparator:"," envDefault:"https://claude.ai/api/mcp/auth_callback,https://claude.com/api/mcp/auth_callback,http://localhost,http://127.0.0.1"`
	McpOAuthAccessTTL    time.Duration `env:"MCP_OAUTH_ACCESS_TTL" envDefault:"1h"`
	McpOAuthRefreshTTL   time.Duration `env:"MCP_OAUTH_REFRESH_TTL" envDefault:"720h"`

	HttpCors bool `env:"HTTP_CORS" envDefault:"false"`
	// HttpCorsAllowedOrigins — белый список Origin для CORS (через запятую).
	// Поддерживаются wildcard-шаблоны вида https://*.example.com. Действует
	// только при HTTP_CORS=true. Если список пуст или содержит "*" —
	// разрешаются любые Origin (нерекомендуемый режим, пишется warning).
	HttpCorsAllowedOrigins []string `env:"HTTP_CORS_ALLOWED_ORIGINS" envSeparator:","`

	PgDsn string `env:"PG_DSN"`

	JWTSecret string `env:"JWT_SECRET"`

	// Серверный ключ HMAC-отпечатков значений (аудит, журнал sync, read-only
	// ответы). Обязателен: без него сервис не стартует (проверка в app.Init).
	// Менять нельзя — старые отпечатки перестанут совпадать с новыми.
	AuditHashKey string `env:"AUDIT_HASH_KEY"`
	// Срок хранения записей аудита и журнала sync, в днях.
	AuditRetentionDays int `env:"AUDIT_RETENTION_DAYS" envDefault:"180"`

	// Префикс имён k8s-секретов, создаваемых kusec: исключает коллизии с
	// уже существующими секретами чартов и даёт плавный переход — старый
	// helm-секрет живёт до передеплоя чарта, kusec-секрет появляется рядом.
	KubeSecretNamePrefix string `env:"KUBE_SECRET_NAME_PREFIX" envDefault:"kusec-"`
}{}

func init() {
	if err := env.Parse(&Conf); err != nil {
		panic(err)
	}
}
