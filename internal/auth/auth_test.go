package auth

import (
  "testing"
  "time"
)

func TestHashAndCheckPassword(t *testing.T) {
  hash, err := HashPassword("super-secret")
  if err != nil {
    t.Fatalf("hash error: %v", err)
  }

  if err := CheckPassword(hash, "super-secret"); err != nil {
    t.Fatalf("expected password to match, got %v", err)
  }

  if err := CheckPassword(hash, "wrong"); err == nil {
    t.Fatal("expected mismatch error")
  }
}

func TestGenerateAndParseToken(t *testing.T) {
  secret := "test-secret"
  claims := Claims{UserID: "u1", TenantID: "t1", RoleID: "r1", RoleName: "HR"}

  token, err := GenerateToken(secret, claims, time.Hour)
  if err != nil {
    t.Fatalf("token error: %v", err)
  }

  parsed, err := ParseToken(secret, token)
  if err != nil {
    t.Fatalf("parse error: %v", err)
  }

  if parsed.UserID != claims.UserID || parsed.TenantID != claims.TenantID || parsed.RoleID != claims.RoleID || parsed.RoleName != claims.RoleName {
    t.Fatalf("claims mismatch: %+v", parsed)
  }
}
