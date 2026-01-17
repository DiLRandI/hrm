package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID         string          `json:"id"`
	ActorID    string          `json:"actorId"`
	Action     string          `json:"action"`
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	RequestID  string          `json:"requestId"`
	IP         string          `json:"ip"`
	CreatedAt  any             `json:"createdAt"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
}

type Filter struct {
	Action     string
	EntityType string
	ActorUser  string
}

type Service struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Service {
	return &Service{DB: db}
}

func (s *Service) Record(ctx context.Context, tenantID, actorID, action, entityType, entityID, requestID, ip string, before, after any) error {
	var beforeJSON, afterJSON []byte
	if before != nil {
		payload, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeJSON = payload
	}
	if after != nil {
		payload, err := json.Marshal(after)
		if err != nil {
			return err
		}
		afterJSON = payload
	}

	_, err := s.DB.Exec(ctx, `
    INSERT INTO audit_events (tenant_id, actor_user_id, action, entity_type, entity_id, before_json, after_json, request_id, ip)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
  `, tenantID, actorID, action, entityType, entityID, beforeJSON, afterJSON, requestID, ip)
	return err
}

func (s *Service) Count(ctx context.Context, tenantID string, filter Filter) (int, error) {
	query, args := s.buildBaseQuery("SELECT COUNT(1)", tenantID, filter)
	var total int
	if err := s.DB.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) List(ctx context.Context, tenantID string, filter Filter, includeDetails bool, limit, offset int) ([]Event, error) {
	selectCols := "id, actor_user_id, action, entity_type, entity_id, request_id, ip, created_at"
	if includeDetails {
		selectCols += ", before_json, after_json"
	}
	query, args := s.buildBaseQuery("SELECT "+selectCols, tenantID, filter)
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", limitPos, offsetPos)
	args = append(args, limit, offset)

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var evt Event
		if includeDetails {
			if err := rows.Scan(&evt.ID, &evt.ActorID, &evt.Action, &evt.EntityType, &evt.EntityID, &evt.RequestID, &evt.IP, &evt.CreatedAt, &evt.Before, &evt.After); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&evt.ID, &evt.ActorID, &evt.Action, &evt.EntityType, &evt.EntityID, &evt.RequestID, &evt.IP, &evt.CreatedAt); err != nil {
				return nil, err
			}
		}
		out = append(out, evt)
	}
	return out, nil
}

func (s *Service) ListExport(ctx context.Context, tenantID string) ([]Event, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, actor_user_id, action, entity_type, entity_id, request_id, ip, created_at
    FROM audit_events
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var evt Event
		if err := rows.Scan(&evt.ID, &evt.ActorID, &evt.Action, &evt.EntityType, &evt.EntityID, &evt.RequestID, &evt.IP, &evt.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, evt)
	}
	return out, nil
}

func (s *Service) buildBaseQuery(prefix, tenantID string, filter Filter) (string, []any) {
	query := prefix + " FROM audit_events WHERE tenant_id = $1"
	args := []any{tenantID}
	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, filter.Action)
	}
	if filter.EntityType != "" {
		query += fmt.Sprintf(" AND entity_type = $%d", len(args)+1)
		args = append(args, filter.EntityType)
	}
	if filter.ActorUser != "" {
		query += fmt.Sprintf(" AND actor_user_id::text = $%d", len(args)+1)
		args = append(args, filter.ActorUser)
	}
	return query, args
}
