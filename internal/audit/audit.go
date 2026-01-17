package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Service {
	return &Service{DB: db}
}

func (s *Service) Record(ctx context.Context, tenantID, actorID, action, entityType, entityID, requestID, ip string, before, after any) error {
	var beforeJSON, afterJSON []byte
	if before != nil {
		beforeJSON, _ = json.Marshal(before)
	}
	if after != nil {
		afterJSON, _ = json.Marshal(after)
	}

	_, err := s.DB.Exec(ctx, `
    INSERT INTO audit_events (tenant_id, actor_user_id, action, entity_type, entity_id, before_json, after_json, request_id, ip)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
  `, tenantID, actorID, action, entityType, entityID, beforeJSON, afterJSON, requestID, ip)
	return err
}
