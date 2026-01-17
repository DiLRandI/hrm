package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	RunMigrations           bool
	RunSeed                 bool
	MaxBodyBytes            int64
	RateLimitPerMinute      int
}

func Load() Config {
	return Config{
		Addr:                    getEnv("APP_ADDR", ":8080"),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		JWTSecret:               getEnv("JWT_SECRET", ""),
		FrontendDir:             getEnv("FRONTEND_DIR", "frontend/dist"),
		Environment:             getEnv("APP_ENV", "development"),
		SeedTenantName:          getEnv("SEED_TENANT_NAME", "Default Tenant"),
		SeedAdminEmail:          getEnv("SEED_ADMIN_EMAIL", ""),
		SeedAdminPassword:       getEnv("SEED_ADMIN_PASSWORD", ""),
		SeedSystemAdminEmail:    getEnv("SEED_SYSTEM_ADMIN_EMAIL", ""),
		SeedSystemAdminPassword: getEnv("SEED_SYSTEM_ADMIN_PASSWORD", ""),
		AllowSelfSignup:         getEnvBool("ALLOW_SELF_SIGNUP", false),
		EmailFrom:               getEnv("EMAIL_FROM", "no-reply@example.com"),
		RunMigrations:           getEnvBool("RUN_MIGRATIONS", true),
		RunSeed:                 getEnvBool("RUN_SEED", true),
		MaxBodyBytes:            int64(getEnvInt("MAX_BODY_BYTES", 1048576)),
		RateLimitPerMinute:      getEnvInt("RATE_LIMIT_PER_MINUTE", 60),
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

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Environment == "production" {
		if strings.TrimSpace(c.JWTSecret) == "" {
			return fmt.Errorf("JWT_SECRET must be set to a strong value in production")
		}
		if c.RunSeed && strings.TrimSpace(c.SeedAdminPassword) == "" {
			return fmt.Errorf("SEED_ADMIN_PASSWORD must be changed or RUN_SEED disabled in production")
		}
	}
	if c.MaxBodyBytes < 1024 {
		return fmt.Errorf("MAX_BODY_BYTES must be at least 1024")
	}
	if c.RateLimitPerMinute <= 0 {
		return fmt.Errorf("RATE_LIMIT_PER_MINUTE must be positive")
	}
	return nil
}
