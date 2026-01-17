package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"hrm/internal/app/server"
	"hrm/internal/platform/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := server.New(ctx, cfg)
	if err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer app.Close()

	slog.Info("server listening", "addr", cfg.Addr)
	if err := app.Run(ctx); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
