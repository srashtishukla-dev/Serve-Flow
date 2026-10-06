package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/serveflow/serveflow/backend/internal/email"
)

type Database struct {
	Host     string
	Port     uint16
	Name     string
	User     string
	Password string
}

type Redis struct {
	Host string
	Port uint16
}

type Config struct {
	HTTPPort          string
	CORSAllowedOrigin string
	JWTSecret         string
	JWTAccessTokenTTL time.Duration
	Database          Database
	Redis             Redis
	Email             email.Settings
}

func Load() (Config, error) {
	port, err := strconv.ParseUint(valueOrDefault("POSTGRES_PORT", "5433"), 10, 16)
	if err != nil || port == 0 {
		return Config{}, fmt.Errorf("POSTGRES_PORT must be a valid port number")
	}
	redisPort, err := strconv.ParseUint(valueOrDefault("REDIS_PORT", "6379"), 10, 16)
	if err != nil || redisPort == 0 {
		return Config{}, fmt.Errorf("REDIS_PORT must be a valid port number")
	}

	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		return Config{}, fmt.Errorf("POSTGRES_PASSWORD must be set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	jwtAccessTokenTTL, err := time.ParseDuration(valueOrDefault("JWT_ACCESS_TOKEN_TTL", "15m"))
	if err != nil || jwtAccessTokenTTL <= 0 {
		return Config{}, fmt.Errorf("JWT_ACCESS_TOKEN_TTL must be a positive duration")
	}
	emailSettings, err := email.LoadSettings(valueOrDefault("APP_ENV", "development"))
	if err != nil {
		return Config{}, fmt.Errorf("email configuration: %w", err)
	}

	return Config{
		HTTPPort:          valueOrDefault("HTTP_PORT", "8080"),
		CORSAllowedOrigin: valueOrDefault("CORS_ALLOWED_ORIGIN", "http://localhost:5173"),
		JWTSecret:         jwtSecret,
		JWTAccessTokenTTL: jwtAccessTokenTTL,
		Database: Database{
			Host:     valueOrDefault("POSTGRES_HOST", "localhost"),
			Port:     uint16(port),
			Name:     valueOrDefault("POSTGRES_DB", "serveflow"),
			User:     valueOrDefault("POSTGRES_USER", "serveflow"),
			Password: password,
		},
		Redis: Redis{
			Host: valueOrDefault("REDIS_HOST", "localhost"),
			Port: uint16(redisPort),
		},
		Email: emailSettings,
	}, nil
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
