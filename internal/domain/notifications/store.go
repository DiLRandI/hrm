package notifications

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	DB *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{DB: db}
}
