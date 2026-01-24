package leave

import "hrm/internal/platform/querier"

type Store struct {
	DB querier.Querier
}

func NewStore(db querier.Querier) *Store {
	return &Store{DB: db}
}
