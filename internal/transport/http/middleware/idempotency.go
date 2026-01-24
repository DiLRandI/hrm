package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyStore struct {
	db *pgxpool.Pool
}

func NewIdempotencyStore(db *pgxpool.Pool) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

func RequestHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *IdempotencyStore) Check(ctx context.Context, tenantID, userID, endpoint, key, requestHash string) (json.RawMessage, bool, error) {
	var stored json.RawMessage
	err := s.db.QueryRow(ctx, `
    SELECT response_json
    FROM idempotency_keys
    WHERE tenant_id = $1 AND user_id = $2 AND key = $3 AND endpoint = $4 AND request_hash = $5
  `, tenantID, userID, key, endpoint, requestHash).Scan(&stored)
	if err != nil {
		return nil, false, nil
	}
	return stored, true, nil
}

func (s *IdempotencyStore) Save(ctx context.Context, tenantID, userID, endpoint, key, requestHash string, response json.RawMessage) error {
	_, err := s.db.Exec(ctx, `
    INSERT INTO idempotency_keys (tenant_id, user_id, key, endpoint, request_hash, response_json)
    VALUES ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (tenant_id, user_id, key, endpoint) DO UPDATE SET response_json = EXCLUDED.response_json
  `, tenantID, userID, key, endpoint, requestHash, response)
	return err
}
