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
	HttpCors bool   `env:"HTTP_CORS" envDefault:"false"`

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
