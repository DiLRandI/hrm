package notifications

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Mailer interface {
	Send(ctx context.Context, from, to, subject, body string) error
}

type Service struct {
	DB          *pgxpool.Pool
	Mailer      Mailer
	DefaultFrom string
}

func New(db *pgxpool.Pool, mailer Mailer) *Service {
	return &Service{DB: db, Mailer: mailer, DefaultFrom: "no-reply@example.com"}
}

func (s *Service) Create(ctx context.Context, tenantID, userID, ntype, title, body string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO notifications (tenant_id, user_id, type, title, body)
    VALUES ($1,$2,$3,$4,$5)
  `, tenantID, userID, ntype, title, body)
	if err != nil {
		return err
	}

	if s.Mailer == nil {
		return nil
	}

	enabled, from := s.getEmailSettings(ctx, tenantID)
	if !enabled {
		return nil
	}
	if from == "" {
		from = s.DefaultFrom
	}

	var email string
	if err := s.DB.QueryRow(ctx, "SELECT email FROM users WHERE tenant_id = $1 AND id = $2", tenantID, userID).Scan(&email); err != nil {
		slog.Warn("notification email lookup failed", "err", err)
		return nil
	}
	if email == "" {
		return nil
	}
	if err := s.Mailer.Send(ctx, from, email, title, body); err != nil {
		slog.Warn("notification email send failed", "err", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context, tenantID, userID string, limit, offset int) ([]map[string]any, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, type, title, body, read_at, created_at
    FROM notifications
    WHERE tenant_id = $1 AND user_id = $2
    ORDER BY created_at DESC
    LIMIT $3 OFFSET $4
  `, tenantID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, ntype, title, body string
		var readAt, createdAt any
		if err := rows.Scan(&id, &ntype, &title, &body, &readAt, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":        id,
			"type":      ntype,
			"title":     title,
			"body":      body,
			"readAt":    readAt,
			"createdAt": createdAt,
		})
	}
	return out, nil
}

func (s *Service) Count(ctx context.Context, tenantID, userID string) (int, error) {
	var total int
	if err := s.DB.QueryRow(ctx, "SELECT COUNT(1) FROM notifications WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) MarkRead(ctx context.Context, tenantID, userID, notificationID string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE notifications SET read_at = now()
    WHERE tenant_id = $1 AND user_id = $2 AND id = $3
  `, tenantID, userID, notificationID)
	return err
}

func (s *Service) getEmailSettings(ctx context.Context, tenantID string) (bool, string) {
	var enabled bool
	var from string
	if err := s.DB.QueryRow(ctx, `
    SELECT email_notifications_enabled, COALESCE(email_from, '')
    FROM tenant_settings
    WHERE tenant_id = $1
  `, tenantID).Scan(&enabled, &from); err == nil {
		return enabled, from
	}
	return false, ""
}

func (s *Service) GetSettings(ctx context.Context, tenantID string) (bool, string, error) {
	var enabled bool
	var from string
	err := s.DB.QueryRow(ctx, `
    SELECT email_notifications_enabled, COALESCE(email_from, '')
    FROM tenant_settings
    WHERE tenant_id = $1
  `, tenantID).Scan(&enabled, &from)
	if err != nil {
		return false, "", err
	}
	return enabled, from, nil
}

func (s *Service) UpdateSettings(ctx context.Context, tenantID string, enabled bool, from string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO tenant_settings (tenant_id, email_notifications_enabled, email_from)
    VALUES ($1,$2,$3)
    ON CONFLICT (tenant_id) DO UPDATE
      SET email_notifications_enabled = EXCLUDED.email_notifications_enabled,
          email_from = EXCLUDED.email_from,
          updated_at = now()
  `, tenantID, enabled, nullIfEmpty(from))
	return err
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
