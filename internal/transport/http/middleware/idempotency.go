package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIdempotencyConflict = errors.New("idempotency key conflicts with existing request")

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
	if s == nil || s.db == nil {
		return nil, false, nil
	}
	var storedHash string
	var stored json.RawMessage
	err := s.db.QueryRow(ctx, `
    SELECT request_hash, response_json
    FROM idempotency_keys
    WHERE tenant_id = $1 AND user_id = $2 AND key = $3 AND endpoint = $4
  `, tenantID, userID, key, endpoint).Scan(&storedHash, &stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedHash != requestHash {
		return nil, false, ErrIdempotencyConflict
	}
	return stored, true, nil
}

func (s *IdempotencyStore) Save(ctx context.Context, tenantID, userID, endpoint, key, requestHash string, response json.RawMessage) error {
	if s == nil || s.db == nil {
		return nil
	}
	tag, err := s.db.Exec(ctx, `
    INSERT INTO idempotency_keys (tenant_id, user_id, key, endpoint, request_hash, response_json)
    VALUES ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (tenant_id, user_id, key, endpoint)
    DO UPDATE SET response_json = EXCLUDED.response_json
    WHERE idempotency_keys.request_hash = EXCLUDED.request_hash
  `, tenantID, userID, key, endpoint, requestHash, response)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrIdempotencyConflict
	}
	return nil
}
