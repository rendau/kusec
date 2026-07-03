package config

import (
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
	HttpCors   bool   `env:"HTTP_CORS" envDefault:"false"`
	// HttpCorsAllowedOrigins — белый список Origin для CORS (через запятую).
	// Поддерживаются wildcard-шаблоны вида https://*.example.com. Действует
	// только при HTTP_CORS=true. Если список пуст или содержит "*" —
	// разрешаются любые Origin (нерекомендуемый режим, пишется warning).
	HttpCorsAllowedOrigins []string `env:"HTTP_CORS_ALLOWED_ORIGINS" envSeparator:","`

	PgDsn string `env:"PG_DSN"`

	JWTSecret string `env:"JWT_SECRET"`

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
