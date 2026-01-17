package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr                    string
	DatabaseURL             string
	JWTSecret               string
	FrontendDir             string
	Environment             string
	SeedTenantName          string
	SeedAdminEmail          string
	SeedAdminPassword       string
	SeedSystemAdminEmail    string
	SeedSystemAdminPassword string
	AllowSelfSignup         bool
	EmailFrom               string
}

func Load() Config {
	return Config{
		Addr:                    getEnv("APP_ADDR", ":8080"),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		JWTSecret:               getEnv("JWT_SECRET", "change-me"),
		FrontendDir:             getEnv("FRONTEND_DIR", "frontend/dist"),
		Environment:             getEnv("APP_ENV", "development"),
		SeedTenantName:          getEnv("SEED_TENANT_NAME", "Default Tenant"),
		SeedAdminEmail:          getEnv("SEED_ADMIN_EMAIL", "admin@example.com"),
		SeedAdminPassword:       getEnv("SEED_ADMIN_PASSWORD", "ChangeMe123!"),
		SeedSystemAdminEmail:    getEnv("SEED_SYSTEM_ADMIN_EMAIL", ""),
		SeedSystemAdminPassword: getEnv("SEED_SYSTEM_ADMIN_PASSWORD", ""),
		AllowSelfSignup:         getEnvBool("ALLOW_SELF_SIGNUP", false),
		EmailFrom:               getEnv("EMAIL_FROM", "no-reply@example.com"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
