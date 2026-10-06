// Package repo is the PostgreSQL persistence layer.
package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/GenJi77JYXC/tinyurl/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver named "pgx"
)

// PostgresRepo stores links in PostgreSQL.
type PostgresRepo struct {
	db *sql.DB
}

// New opens a pooled *sql.DB backed by the pgx driver and applies sane
// connection-pool defaults. Callers must call Close on shutdown.
func New(databaseURL string) (*PostgresRepo, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return &PostgresRepo{db: db}, nil
}

// Ping verifies connectivity.
func (r *PostgresRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the underlying connection pool.
func (r *PostgresRepo) Close() error { return r.db.Close() }

// NextID allocates the next value from the links_id_seq sequence.
// sequence allocation is non-transactional: a failed insert leaves a gap,
// which is harmless for short codes.
func (r *PostgresRepo) NextID(ctx context.Context) (int64, error) {
	var id int64
	if err := r.db.QueryRowContext(ctx, "SELECT nextval('links_id_seq')").Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// InsertLink persists a link whose ID/code were allocated by the service.
func (r *PostgresRepo) InsertLink(ctx context.Context, id int64, code, originalURL string) error {
	const q = `INSERT INTO links (id, short_code, original_url) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, q, id, code, originalURL)
	return err
}

// GetURLByCode returns the original URL for a code, or model.ErrNotFound.
func (r *PostgresRepo) GetURLByCode(ctx context.Context, code string) (string, error) {
	const q = `SELECT original_url FROM links WHERE short_code = $1`
	var originalURL string
	err := r.db.QueryRowContext(ctx, q, code).Scan(&originalURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", model.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return originalURL, nil
}
