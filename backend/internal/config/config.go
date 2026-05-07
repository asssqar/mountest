package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AppPort       string
	DatabaseURL   string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
	SeedDemo      bool
	CORSOrigin    string
	CookieSecure  bool
}

func Load() Config {
	cfg := Config{
		AppPort:       getenv("APP_PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     getenv("JWT_SECRET", "change-me"),
		AdminUsername: getenv("ADMIN_USERNAME", "admin"),
		AdminPassword: getenv("ADMIN_PASSWORD", "admin123"),
		SeedDemo:      getbool("SEED_DEMO", true),
		CORSOrigin:    getenv("CORS_ORIGIN", "http://localhost:5173"),
		CookieSecure:  getbool("COOKIE_SECURE", false),
	}
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			getenv("POSTGRES_USER", "mountest"),
			getenv("POSTGRES_PASSWORD", "mountest"),
			getenv("POSTGRES_HOST", "localhost"),
			getenv("POSTGRES_PORT", "5432"),
			getenv("POSTGRES_DB", "mountest"),
		)
	}
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
