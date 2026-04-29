package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// pgxStore implements all DeliveryStore methods against a querier.
// Used directly (pool) and inside transactions (pgx.Tx).
type pgxStore struct {
	q querier
}

// PostgresStore wraps pgxStore and adds transaction support.
type PostgresStore struct {
	pgxStore
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, pool *pgxpool.Pool) (*PostgresStore, error) {
	if err := migrate(ctx, pool); err != nil {
		return nil, err
	}
	return &PostgresStore{pgxStore: pgxStore{q: pool}, pool: pool}, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	const sql = `
		CREATE TABLE IF NOT EXISTS deliveries (
			id              TEXT        PRIMARY KEY,
			event_id        TEXT        NOT NULL,
			subscription_id TEXT        NOT NULL,
			destination_url TEXT        NOT NULL,
			method          TEXT        NOT NULL,
			headers         JSONB       NOT NULL DEFAULT '{}',
			payload         BYTEA       NOT NULL DEFAULT '',
			status          TEXT        NOT NULL DEFAULT 'pending',
			attempts        INT         NOT NULL DEFAULT 0,
			next_attempt    TIMESTAMPTZ,
			last_error      TEXT        NOT NULL DEFAULT '',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE deliveries ADD COLUMN IF NOT EXISTS headers      JSONB       NOT NULL DEFAULT '{}';
		ALTER TABLE deliveries ADD COLUMN IF NOT EXISTS payload      BYTEA       NOT NULL DEFAULT '';
		ALTER TABLE deliveries ADD COLUMN IF NOT EXISTS next_attempt TIMESTAMPTZ;
		CREATE INDEX IF NOT EXISTS idx_deliveries_event_id ON deliveries (event_id);
		CREATE INDEX IF NOT EXISTS idx_deliveries_pending  ON deliveries (next_attempt) WHERE status = 'pending';
	`
	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("migrate deliveries: %w", err)
	}
	return nil
}

func (s *PostgresStore) Transact(ctx context.Context, fn func(DeliveryStore) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := fn(&pgxStore{q: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// pgxStore.Transact satisfies the interface inside a transaction context (no nested tx).
func (s *pgxStore) Transact(_ context.Context, fn func(DeliveryStore) error) error {
	return fn(s)
}

func (s *pgxStore) Create(ctx context.Context, r DeliveryRecord) error {
	headersJSON, err := json.Marshal(r.Headers)
	if err != nil {
		return fmt.Errorf("marshal headers: %w", err)
	}
	const q = `
		INSERT INTO deliveries (id, event_id, subscription_id, destination_url, method, headers, payload, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`
	_, err = s.q.Exec(ctx, q, r.ID, r.EventID, r.SubscriptionID, r.DestinationURL, r.Method, headersJSON, r.Payload, r.Status)
	return err
}

func (s *pgxStore) GetPendingLocked(ctx context.Context, limit int) ([]DeliveryRecord, error) {
	const q = `
		SELECT id, event_id, subscription_id, destination_url, method, headers, payload, attempts, last_error
		FROM deliveries
		WHERE status = 'pending' AND next_attempt <= NOW()
		ORDER BY next_attempt
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := s.q.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DeliveryRecord
	for rows.Next() {
		var r DeliveryRecord
		var headersJSON []byte
		if err := rows.Scan(&r.ID, &r.EventID, &r.SubscriptionID, &r.DestinationURL, &r.Method, &headersJSON, &r.Payload, &r.Attempts, &r.LastError); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(headersJSON, &r.Headers); err != nil {
			return nil, fmt.Errorf("unmarshal headers for %s: %w", r.ID, err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *pgxStore) Reschedule(ctx context.Context, id string, attempts int, nextAttempt time.Time, lastError string) error {
	const q = `
		UPDATE deliveries
		SET attempts = $2, next_attempt = $3, last_error = $4, updated_at = NOW()
		WHERE id = $1
	`
	_, err := s.q.Exec(ctx, q, id, attempts, nextAttempt, lastError)
	return err
}

func (s *pgxStore) MarkSuccess(ctx context.Context, id string) error {
	const q = `
		UPDATE deliveries
		SET status = 'success', next_attempt = NULL, updated_at = NOW()
		WHERE id = $1
	`
	_, err := s.q.Exec(ctx, q, id)
	return err
}

func (s *pgxStore) MarkExhausted(ctx context.Context, id string, lastError string) error {
	const q = `
		UPDATE deliveries
		SET status = 'exhausted', next_attempt = NULL, last_error = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := s.q.Exec(ctx, q, id, lastError)
	return err
}

func (s *pgxStore) CountPending(ctx context.Context) (int, error) {
	var count int
	err := s.q.QueryRow(ctx, `SELECT COUNT(*) FROM deliveries WHERE status = 'pending'`).Scan(&count)
	return count, err
}

func (s *pgxStore) ListDeliveries(ctx context.Context, status string, limit, offset int) ([]DeliveryRecord, error) {
	const q = `
		SELECT id, event_id, subscription_id, destination_url, method, status,
		       attempts, next_attempt, last_error, created_at, updated_at
		FROM deliveries
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.q.Query(ctx, q, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DeliveryRecord
	for rows.Next() {
		var r DeliveryRecord
		if err := rows.Scan(
			&r.ID, &r.EventID, &r.SubscriptionID, &r.DestinationURL, &r.Method, &r.Status,
			&r.Attempts, &r.NextAttempt, &r.LastError, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *pgxStore) StatusStats(ctx context.Context) (map[Status]int, error) {
	rows, err := s.q.Query(ctx, `SELECT status, COUNT(*) FROM deliveries GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[Status]int{
		StatusPending:   0,
		StatusSuccess:   0,
		StatusExhausted: 0,
	}
	for rows.Next() {
		var status Status
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}
	return stats, rows.Err()
}
