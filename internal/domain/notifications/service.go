package notifications

import (
	"context"
	"log/slog"
)

type Mailer interface {
	Send(ctx context.Context, from, to, subject, body string) error
}

type Service struct {
	store       StoreAPI
	Mailer      Mailer
	DefaultFrom string
}

func New(store StoreAPI, mailer Mailer) *Service {
	return &Service{store: store, Mailer: mailer, DefaultFrom: "no-reply@example.com"}
}

func (s *Service) Create(ctx context.Context, tenantID, userID, ntype, title, body string) error {
	if err := s.store.CreateNotification(ctx, tenantID, userID, ntype, title, body); err != nil {
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

	email, err := s.store.UserEmail(ctx, tenantID, userID)
	if err != nil {
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
	return s.store.ListNotifications(ctx, tenantID, userID, limit, offset)
}

func (s *Service) Count(ctx context.Context, tenantID, userID string) (int, error) {
	return s.store.CountNotifications(ctx, tenantID, userID)
}

func (s *Service) MarkRead(ctx context.Context, tenantID, userID, notificationID string) error {
	return s.store.MarkRead(ctx, tenantID, userID, notificationID)
}

func (s *Service) getEmailSettings(ctx context.Context, tenantID string) (bool, string) {
	enabled, from, err := s.store.EmailSettings(ctx, tenantID)
	if err != nil {
		return false, ""
	}
	return enabled, from
}

func (s *Service) GetSettings(ctx context.Context, tenantID string) (bool, string, error) {
	return s.store.EmailSettings(ctx, tenantID)
}

func (s *Service) UpdateSettings(ctx context.Context, tenantID string, enabled bool, from string) error {
	return s.store.UpdateSettings(ctx, tenantID, enabled, from)
}
