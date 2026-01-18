package performance

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Store *Store
}

func NewService(store *Store) *Service {
	return &Service{Store: store}
}

func (s *Service) Pool() *pgxpool.Pool {
	return s.Store.DB
}

func (s *Service) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return s.Store.DB.Query(ctx, sql, args...)
}

func (s *Service) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return s.Store.DB.QueryRow(ctx, sql, args...)
}

func (s *Service) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return s.Store.DB.Exec(ctx, sql, args...)
}

func (s *Service) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.Store.DB.Begin(ctx)
}
