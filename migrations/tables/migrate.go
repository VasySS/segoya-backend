// Package tables contains logic to run table migrations on Postgres.
package tables

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const migrationsDir = "."

//go:embed *.sql
var sqlMigrations embed.FS

// RunGooseMigrations runs goose migrations on Postgres using the embedded migrations.
func RunGooseMigrations(ctx context.Context, pool *pgxpool.Pool, command string, args ...string) error {
	goose.SetBaseFS(sqlMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.RunContext(ctx, command,
		stdlib.OpenDBFromPool(pool),
		migrationsDir,
		args...,
	); err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	return nil
}
