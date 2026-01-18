package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	DB *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{DB: db}
}

type AuthUser struct {
	ID          string
	TenantID    string
	RoleID      string
	RoleName    string
	Password    string
	MFAEnabled  bool
	MFASecretEn []byte
}

func (s *Store) FindActiveUserByEmail(ctx context.Context, email, status string) (AuthUser, error) {
	var out AuthUser
	err := s.DB.QueryRow(ctx, `
    SELECT u.id, u.tenant_id, u.role_id, r.name, u.password_hash, u.mfa_enabled, u.mfa_secret_enc
    FROM users u
    JOIN roles r ON u.role_id = r.id
    WHERE u.email = $1 AND u.status = $2
  `, email, status).Scan(&out.ID, &out.TenantID, &out.RoleID, &out.RoleName, &out.Password, &out.MFAEnabled, &out.MFASecretEn)
	return out, err
}

func (s *Store) CreateSession(ctx context.Context, userID, refreshTokenHash string, expires time.Time) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO sessions (user_id, refresh_token, expires_at)
    VALUES ($1,$2,$3)
  `, userID, refreshTokenHash, expires)
	return err
}

func (s *Store) UpdateLastLogin(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx, "UPDATE users SET last_login = now() WHERE id = $1", userID)
	return err
}

func (s *Store) RevokeSession(ctx context.Context, userID, refreshTokenHash string) error {
	_, err := s.DB.Exec(ctx, "UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND refresh_token = $2", userID, refreshTokenHash)
	return err
}

func (s *Store) SessionValid(ctx context.Context, userID, refreshTokenHash string) (bool, error) {
	var count int
	if err := s.DB.QueryRow(ctx, `
    SELECT COUNT(1)
    FROM sessions
    WHERE user_id = $1 AND refresh_token = $2 AND expires_at > now() AND revoked_at IS NULL
  `, userID, refreshTokenHash).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) RotateSession(ctx context.Context, userID, oldHash, newHash string, expires time.Time) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE sessions
    SET refresh_token = $1, expires_at = $2, rotated_at = now()
    WHERE user_id = $3 AND refresh_token = $4
  `, newHash, expires, userID, oldHash)
	return err
}

func (s *Store) UpdateMFASecret(ctx context.Context, userID string, secretEnc []byte) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE users SET mfa_secret_enc = $1, mfa_enabled = false WHERE id = $2
  `, secretEnc, userID)
	return err
}

func (s *Store) GetMFASecret(ctx context.Context, userID string) ([]byte, error) {
	var secretEnc []byte
	if err := s.DB.QueryRow(ctx, "SELECT mfa_secret_enc FROM users WHERE id = $1", userID).Scan(&secretEnc); err != nil {
		return nil, err
	}
	return secretEnc, nil
}

func (s *Store) SetMFAEnabled(ctx context.Context, userID string, enabled bool) error {
	_, err := s.DB.Exec(ctx, "UPDATE users SET mfa_enabled = $1 WHERE id = $2", enabled, userID)
	return err
}

func (s *Store) UserIDByEmail(ctx context.Context, email string) (string, error) {
	var userID string
	if err := s.DB.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Store) CreatePasswordReset(ctx context.Context, userID, tokenHash string, expires time.Time) error {
	_, err := s.DB.Exec(ctx, "INSERT INTO password_resets (user_id, token, expires_at) VALUES ($1, $2, $3)", userID, tokenHash, expires)
	return err
}

func (s *Store) PasswordResetUserID(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := s.DB.QueryRow(ctx, `
    SELECT user_id
    FROM password_resets
    WHERE token = $1 AND expires_at > now() AND used_at IS NULL
  `, tokenHash).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID, hash string) error {
	_, err := s.DB.Exec(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", hash, userID)
	return err
}

func (s *Store) MarkPasswordResetUsed(ctx context.Context, tokenHash string) error {
	_, err := s.DB.Exec(ctx, "UPDATE password_resets SET used_at = now() WHERE token = $1", tokenHash)
	return err
}
