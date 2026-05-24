// Package db — PostgreSQL pgx pool + Redis client.
package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	PG    *pgxpool.Pool
	Redis *redis.Client
}

func Connect(ctx context.Context) (*Deps, error) {
	pg, err := connectPG(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	rc, err := connectRedis(ctx)
	if err != nil {
		pg.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	return &Deps{PG: pg, Redis: rc}, nil
}

func connectPG(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"),
		os.Getenv("DB_PASSWORD"),
		envOr("DB_NAME", "ward_duty"),
		envOr("DB_SSLMODE", "disable"),
	)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func connectRedis(ctx context.Context) (*redis.Client, error) {
	addr := envOr("REDIS_HOST", "localhost:6379")
	rc := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := rc.Ping(ctx).Err(); err != nil {
		_ = rc.Close()
		return nil, err
	}
	return rc, nil
}

func (d *Deps) Close() {
	if d.PG != nil {
		d.PG.Close()
	}
	if d.Redis != nil {
		_ = d.Redis.Close()
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
