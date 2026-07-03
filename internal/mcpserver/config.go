package mcpserver

import (
	"errors"
	"fmt"

	"github.com/caarlos0/env/v9"
)

type Config struct {
	// ApiURL — базовый URL HTTP API kusec, включая префикс /api,
	// например https://kusec.example.com/api.
	ApiURL string `env:"KUSEC_MCP_API_URL,required"`

	// Учётные данные (в порядке предпочтения): API-ключ (ksk_…, создаётся
	// через POST /api-key — рекомендуемый способ), либо долгоживущий
	// refresh-токен, либо логин/пароль сервисного аккаунта без 2FA.
	ApiKey       string `env:"KUSEC_MCP_API_KEY"`
	RefreshToken string `env:"KUSEC_MCP_REFRESH_TOKEN"`
	Username     string `env:"KUSEC_MCP_USERNAME"`
	Password     string `env:"KUSEC_MCP_PASSWORD"`

	InsecureSkipVerify bool `env:"KUSEC_MCP_INSECURE_SKIP_VERIFY" envDefault:"false"`
}

func LoadConfig() (Config, error) {
	cfg := Config{}

	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("parse env: %w", err)
	}

	if cfg.ApiKey == "" && cfg.RefreshToken == "" && (cfg.Username == "" || cfg.Password == "") {
		return cfg, errors.New("задайте KUSEC_MCP_API_KEY, либо KUSEC_MCP_REFRESH_TOKEN, либо KUSEC_MCP_USERNAME и KUSEC_MCP_PASSWORD")
	}

	return cfg, nil
}
