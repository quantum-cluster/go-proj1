package config

import (
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
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/postgres" // default purely for dev
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "not a real secret" // default purely for dev
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051" // default
	}

	return &Config{
		DatabaseURL: dbURL,
		JWTSecret:   jwtSecret,
		GRPCPort:    grpcPort,
	}
}
