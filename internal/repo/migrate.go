package repo

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed migrations/001_init.sql
var migration001 string

// Migrate applies all idempotent DDL statements at startup.
func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, migration001)
	return err
}

// Migrate is a convenience method on the repository.
func (r *PostgresRepo) Migrate(ctx context.Context) error {
	return Migrate(ctx, r.db)
}
