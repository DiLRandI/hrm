package notifications

import (
  "context"

  "github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
  DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Service {
  return &Service{DB: db}
}

func (s *Service) Create(ctx context.Context, tenantID, userID, ntype, title, body string) error {
  _, err := s.DB.Exec(ctx, `
    INSERT INTO notifications (tenant_id, user_id, type, title, body)
    VALUES ($1,$2,$3,$4,$5)
  `, tenantID, userID, ntype, title, body)
  return err
}

func (s *Service) List(ctx context.Context, tenantID, userID string) ([]map[string]interface{}, error) {
  rows, err := s.DB.Query(ctx, `
    SELECT id, type, title, body, read_at, created_at
    FROM notifications
    WHERE tenant_id = $1 AND user_id = $2
    ORDER BY created_at DESC
  `, tenantID, userID)
  if err != nil {
    return nil, err
  }
  defer rows.Close()

  var out []map[string]interface{}
  for rows.Next() {
    var id, ntype, title, body string
    var readAt, createdAt interface{}
    if err := rows.Scan(&id, &ntype, &title, &body, &readAt, &createdAt); err != nil {
      return nil, err
    }
    out = append(out, map[string]interface{}{
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
