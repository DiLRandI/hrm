package main

import (
	"context"
	"log"

	"hrm/internal/app/server"
	"hrm/internal/platform/config"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	app, err := server.New(ctx, cfg)
	if err != nil {
		log.Fatalf("startup failed: %v", err)
	}
	defer app.Close()

	log.Printf("HRM server listening on %s", cfg.Addr)
	if err := app.Run(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
