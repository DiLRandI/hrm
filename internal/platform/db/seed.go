package db

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/auth"
	"hrm/internal/platform/config"
)

func Seed(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	tenantID, err := ensureTenant(ctx, pool, cfg.SeedTenantName)
	if err != nil {
		return err
	}

	if err := ensurePermissions(ctx, pool); err != nil {
		return err
	}

	roleIDs, err := ensureRoles(ctx, pool, tenantID)
	if err != nil {
		return err
	}

	if err := ensureRolePermissions(ctx, pool, roleIDs); err != nil {
		return err
	}

	if err := ensureAdminUser(ctx, pool, tenantID, roleIDs[auth.RoleHR], cfg.SeedAdminEmail, cfg.SeedAdminPassword); err != nil {
		return err
	}

	if cfg.SeedSystemAdminEmail != "" {
		_ = ensureAdminUser(ctx, pool, tenantID, roleIDs[auth.RoleSystemAdmin], cfg.SeedSystemAdminEmail, cfg.SeedSystemAdminPassword)
	}

	return nil
}

func ensureTenant(ctx context.Context, pool *pgxpool.Pool, name string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM tenants WHERE name = $1", name).Scan(&id)
	if err == nil {
		return id, nil
	}

	err = pool.QueryRow(ctx, "INSERT INTO tenants (name) VALUES ($1) RETURNING id", name).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func ensurePermissions(ctx context.Context, pool *pgxpool.Pool) error {
	for _, perm := range auth.DefaultPermissions {
		_, err := pool.Exec(ctx, "INSERT INTO permissions (key) VALUES ($1) ON CONFLICT (key) DO NOTHING", perm)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureRoles(ctx context.Context, pool *pgxpool.Pool, tenantID string) (map[string]string, error) {
	roleIDs := map[string]string{}
	for roleName := range auth.RolePermissions {
		var id string
		err := pool.QueryRow(ctx, "SELECT id FROM roles WHERE tenant_id = $1 AND name = $2", tenantID, roleName).Scan(&id)
		if err == nil {
			roleIDs[roleName] = id
			continue
		}

		err = pool.QueryRow(ctx, "INSERT INTO roles (tenant_id, name) VALUES ($1, $2) RETURNING id", tenantID, roleName).Scan(&id)
		if err != nil {
			return nil, err
		}
		roleIDs[roleName] = id
	}
	return roleIDs, nil
}

func ensureRolePermissions(ctx context.Context, pool *pgxpool.Pool, roleIDs map[string]string) error {
	permMap := map[string]string{}
	rows, err := pool.Query(ctx, "SELECT id, key FROM permissions")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return err
		}
		permMap[key] = id
	}

	for roleName, perms := range auth.RolePermissions {
		roleID := roleIDs[roleName]
		for _, permKey := range perms {
			permID, ok := permMap[permKey]
			if !ok {
				return errors.New("permission not found: " + permKey)
			}
			_, err := pool.Exec(ctx, "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", roleID, permID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureAdminUser(ctx context.Context, pool *pgxpool.Pool, tenantID, roleID, email, password string) error {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		return nil
	}

	var id string
	err := pool.QueryRow(ctx, "SELECT id FROM users WHERE tenant_id = $1 AND email = $2", tenantID, email).Scan(&id)
	if err == nil {
		return nil
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	err = pool.QueryRow(ctx, "INSERT INTO users (tenant_id, email, password_hash, role_id) VALUES ($1, $2, $3, $4) RETURNING id", tenantID, email, hash, roleID).Scan(&id)
	if err != nil {
		return err
	}
	return nil
}
