package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds database connection configuration.
type PoolConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	PoolSize int
	SSLMode  string // disable, require, verify-ca, verify-full
}

// ConnectionString returns a PostgreSQL connection string.
func (c PoolConfig) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

// NewPool creates a new connection pool with the given configuration.
func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	if cfg.PoolSize > 0 {
		poolCfg.MaxConns = int32(cfg.PoolSize)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
