package auth

import (
	"context"
	"time"
)

type Service struct {
	Store *Store
}

func NewService(store *Store) *Service {
	return &Service{Store: store}
}

func (s *Service) FindActiveUserByEmail(ctx context.Context, email, status string) (AuthUser, error) {
	return s.Store.FindActiveUserByEmail(ctx, email, status)
}

func (s *Service) CreateSession(ctx context.Context, userID, refreshTokenHash string, expires time.Time) error {
	return s.Store.CreateSession(ctx, userID, refreshTokenHash, expires)
}

func (s *Service) UpdateLastLogin(ctx context.Context, userID string) error {
	return s.Store.UpdateLastLogin(ctx, userID)
}

func (s *Service) RevokeSession(ctx context.Context, userID, refreshTokenHash string) error {
	return s.Store.RevokeSession(ctx, userID, refreshTokenHash)
}

func (s *Service) SessionValid(ctx context.Context, userID, refreshTokenHash string) (bool, error) {
	return s.Store.SessionValid(ctx, userID, refreshTokenHash)
}

func (s *Service) RotateSession(ctx context.Context, userID, oldHash, newHash string, expires time.Time) error {
	return s.Store.RotateSession(ctx, userID, oldHash, newHash, expires)
}

func (s *Service) UpdateMFASecret(ctx context.Context, userID string, secretEnc []byte) error {
	return s.Store.UpdateMFASecret(ctx, userID, secretEnc)
}

func (s *Service) GetMFASecret(ctx context.Context, userID string) ([]byte, error) {
	return s.Store.GetMFASecret(ctx, userID)
}

func (s *Service) SetMFAEnabled(ctx context.Context, userID string, enabled bool) error {
	return s.Store.SetMFAEnabled(ctx, userID, enabled)
}

func (s *Service) UserIDByEmail(ctx context.Context, email string) (string, error) {
	return s.Store.UserIDByEmail(ctx, email)
}

func (s *Service) CreatePasswordReset(ctx context.Context, userID, tokenHash string, expires time.Time) error {
	return s.Store.CreatePasswordReset(ctx, userID, tokenHash, expires)
}

func (s *Service) PasswordResetUserID(ctx context.Context, tokenHash string) (string, error) {
	return s.Store.PasswordResetUserID(ctx, tokenHash)
}

func (s *Service) UpdateUserPassword(ctx context.Context, userID, hash string) error {
	return s.Store.UpdateUserPassword(ctx, userID, hash)
}

func (s *Service) MarkPasswordResetUsed(ctx context.Context, tokenHash string) error {
	return s.Store.MarkPasswordResetUsed(ctx, tokenHash)
}
