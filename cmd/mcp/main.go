// MCP-сервер kusec для AI-агентов (stdio transport).
//
// Агент видит структуру app/secret/configmap/item, но не значения секретов:
// значения маскируются, а новые задаются декларативно через value_source.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mechta-market/kusec/internal/mcpserver"
)

func main() {
	// stdout занят MCP-транспортом — логи только в stderr
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := mcpserver.LoadConfig()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err = mcpserver.New(cfg).Run(ctx); err != nil {
		slog.Error("mcp server", "error", err)
		os.Exit(1)
	}
}
