package db

import (
  "context"
  "fmt"
  "os"
  "path/filepath"
  "sort"
  "strings"

  "github.com/jackc/pgx/v5"
  "github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
  if err := ensureMigrationsTable(ctx, pool); err != nil {
    return err
  }

  entries, err := os.ReadDir(migrationsDir)
  if err != nil {
    return err
  }

  var files []string
  for _, entry := range entries {
    if entry.IsDir() {
      continue
    }
    if strings.HasSuffix(entry.Name(), ".sql") {
      files = append(files, entry.Name())
    }
  }
  sort.Strings(files)

  for _, file := range files {
    version := strings.TrimSuffix(file, ".sql")
    applied, err := migrationApplied(ctx, pool, version)
    if err != nil {
      return err
    }
    if applied {
      continue
    }

    path := filepath.Join(migrationsDir, file)
    sqlBytes, err := os.ReadFile(path)
    if err != nil {
      return err
    }

    tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
    if err != nil {
      return err
    }

    if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
      _ = tx.Rollback(ctx)
      return fmt.Errorf("migration %s failed: %w", version, err)
    }

    if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
      _ = tx.Rollback(ctx)
      return err
    }

    if err := tx.Commit(ctx); err != nil {
      return err
    }
  }

  return nil
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
  _, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())")
  return err
}

func migrationApplied(ctx context.Context, pool *pgxpool.Pool, version string) (bool, error) {
  var count int
  err := pool.QueryRow(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE version = $1", version).Scan(&count)
  if err != nil {
    return false, err
  }
  return count > 0, nil
}
