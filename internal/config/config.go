package config

import (
	"cmp"
	"os"
)

// Config holds the application configuration
type Config struct {
	DatabaseURL string
	JWTSecret   string
	GRPCPort    string
}

// Load loads the configuration from environment variables, using defaults if not set.
func Load() *Config {
	return &Config{
		DatabaseURL: cmp.Or(os.Getenv("DATABASE_URL"), "postgres://postgres:postgres@localhost:5432/postgres"),
		JWTSecret:   cmp.Or(os.Getenv("JWT_SECRET"), "not a real secret"),
		GRPCPort:    cmp.Or(os.Getenv("GRPC_PORT"), "50051"),
	}
}
