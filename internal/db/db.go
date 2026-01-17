package db

import (
  "context"
  "time"

  "github.com/jackc/pgx/v5/pgxpool"

  "hrm/internal/config"
)

func Connect(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
  poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
  if err != nil {
    return nil, err
  }
  poolCfg.MaxConnLifetime = time.Hour
  poolCfg.MaxConns = 10
  poolCfg.MinConns = 2
  return pgxpool.NewWithConfig(ctx, poolCfg)
}
