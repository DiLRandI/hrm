package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                    string
	DatabaseURL             string
	JWTSecret               string
	DataEncryptionKey       string
	FrontendDir             string
	Environment             string
	SeedTenantName          string
	SeedAdminEmail          string
	SeedAdminPassword       string
	SeedSystemAdminEmail    string
	SeedSystemAdminPassword string
	AllowSelfSignup         bool
	EmailFrom               string
	EmailEnabled            bool
	SMTPHost                string
	SMTPPort                int
	SMTPUser                string
	SMTPPassword            string
	SMTPUseTLS              bool
	RunMigrations           bool
	RunSeed                 bool
	MaxBodyBytes            int64
	RateLimitPerMinute      int
	LeaveAccrualInterval    time.Duration
	RetentionInterval       time.Duration
	MetricsEnabled          bool
}

func Load() Config {
	return Config{
		Addr:                    getEnv("APP_ADDR", ":8080"),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		JWTSecret:               getEnv("JWT_SECRET", ""),
		DataEncryptionKey:       getEnv("DATA_ENCRYPTION_KEY", ""),
		FrontendDir:             getEnv("FRONTEND_DIR", "frontend/dist"),
		Environment:             getEnv("APP_ENV", "development"),
		SeedTenantName:          getEnv("SEED_TENANT_NAME", "Default Tenant"),
		SeedAdminEmail:          getEnv("SEED_ADMIN_EMAIL", ""),
		SeedAdminPassword:       getEnv("SEED_ADMIN_PASSWORD", ""),
		SeedSystemAdminEmail:    getEnv("SEED_SYSTEM_ADMIN_EMAIL", ""),
		SeedSystemAdminPassword: getEnv("SEED_SYSTEM_ADMIN_PASSWORD", ""),
		AllowSelfSignup:         getEnvBool("ALLOW_SELF_SIGNUP", false),
		EmailFrom:               getEnv("EMAIL_FROM", "no-reply@example.com"),
		EmailEnabled:            getEnvBool("EMAIL_ENABLED", false),
		SMTPHost:                getEnv("SMTP_HOST", ""),
		SMTPPort:                getEnvInt("SMTP_PORT", 587),
		SMTPUser:                getEnv("SMTP_USER", ""),
		SMTPPassword:            getEnv("SMTP_PASSWORD", ""),
		SMTPUseTLS:              getEnvBool("SMTP_USE_TLS", true),
		RunMigrations:           getEnvBool("RUN_MIGRATIONS", true),
		RunSeed:                 getEnvBool("RUN_SEED", true),
		MaxBodyBytes:            int64(getEnvInt("MAX_BODY_BYTES", 1048576)),
		RateLimitPerMinute:      getEnvInt("RATE_LIMIT_PER_MINUTE", 60),
		LeaveAccrualInterval:    getEnvDuration("LEAVE_ACCRUAL_INTERVAL", 24*time.Hour),
		RetentionInterval:       getEnvDuration("RETENTION_INTERVAL", 24*time.Hour),
		MetricsEnabled:          getEnvBool("METRICS_ENABLED", true),
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

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
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
		if strings.TrimSpace(c.DataEncryptionKey) == "" {
			return fmt.Errorf("DATA_ENCRYPTION_KEY must be set in production for encryption at rest")
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
	if c.EmailEnabled && c.SMTPHost == "" {
		return fmt.Errorf("SMTP_HOST must be set when EMAIL_ENABLED is true")
	}
	return nil
}
