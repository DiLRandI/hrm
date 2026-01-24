package notifications

import "context"

type StoreAPI interface {
	CreateNotification(ctx context.Context, tenantID, userID, ntype, title, body string) error
	UserEmail(ctx context.Context, tenantID, userID string) (string, error)
	ListNotifications(ctx context.Context, tenantID, userID string, limit, offset int) ([]map[string]any, error)
	CountNotifications(ctx context.Context, tenantID, userID string) (int, error)
	MarkRead(ctx context.Context, tenantID, userID, notificationID string) error
	EmailSettings(ctx context.Context, tenantID string) (bool, string, error)
	UpdateSettings(ctx context.Context, tenantID string, enabled bool, from string) error
}
