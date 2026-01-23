package gdpr

import (
	"context"
	"time"

	"hrm/internal/domain/core"
	cryptoutil "hrm/internal/platform/crypto"
)

type EmployeeStore interface {
	GetEmployee(ctx context.Context, tenantID, employeeID string) (*core.Employee, error)
}

type Service struct {
	store     *Store
	employees EmployeeStore
	crypto    *cryptoutil.Service
}

func NewService(store *Store, employees EmployeeStore, crypto *cryptoutil.Service) *Service {
	return &Service{store: store, employees: employees, crypto: crypto}
}

func (s *Service) ApplyRetention(ctx context.Context, tenantID, category string, cutoff time.Time) (int64, error) {
	return ApplyRetention(ctx, s.store.DB, tenantID, category, cutoff)
}
