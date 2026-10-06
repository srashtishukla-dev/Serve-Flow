package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/config"
)

func Open(ctx context.Context, settings config.Database) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}

	poolConfig.ConnConfig.Host = settings.Host
	poolConfig.ConnConfig.Port = settings.Port
	poolConfig.ConnConfig.Database = settings.Name
	poolConfig.ConnConfig.User = settings.User
	poolConfig.ConnConfig.Password = settings.Password
	for _, fallback := range poolConfig.ConnConfig.Fallbacks {
		fallback.Host = settings.Host
		fallback.Port = settings.Port
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	return pool, nil
}
