package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"reserva/config"
)

// querier is satisfied by both *pgxpool.Pool and pgx.Tx, enabling
// repositories to transparently work inside or outside a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKeyType struct{}

var txKey = txKeyType{}

// PostgresTransactor implements ports.Transactor using a pgxpool.
type PostgresTransactor struct {
	db *pgxpool.Pool
}

func NewPostgresTransactor(db *pgxpool.Pool) *PostgresTransactor {
	return &PostgresTransactor{db: db}
}

// WithinTransaction begins a transaction, injects it into the context, runs fn,
// and commits on success or rolls back on error.
func (t *PostgresTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit; safe to always defer

	ctx = context.WithValue(ctx, txKey, tx)
	if err := fn(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// getQuerier returns the transaction stored in ctx if present,
// otherwise falls back to the pool. All repository methods must call this.
func getQuerier(ctx context.Context, db *pgxpool.Pool) querier {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return tx
	}
	return db
}

func NewPostgresDB(cfg *config.Config) (*pgxpool.Pool, error) {
	// Use url.URL to properly encode username and password — special characters
	// like @, /, or ? in credentials would corrupt the DSN string otherwise.
	connURL := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:     fmt.Sprintf("%s:%s", cfg.DBHost, cfg.DBPort),
		Path:     cfg.DBName,
		RawQuery: "sslmode=" + cfg.DBSSLMode,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbconfig, err := pgxpool.ParseConfig(connURL.String())
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Connection Pool Settings
	dbconfig.MaxConns = 50
	dbconfig.MinConns = 10
	dbconfig.MaxConnLifetime = time.Hour
	dbconfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, dbconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}
