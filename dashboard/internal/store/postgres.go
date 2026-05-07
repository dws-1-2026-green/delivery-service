package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}

func (s *postgresStore) ListDeliveries(ctx context.Context, status, eventID string, limit, offset int) ([]DeliveryRecord, error) {
	const q = `
		SELECT id, event_id, subscription_id, destination_url, method, status,
		       attempts, next_attempt, last_error, created_at, updated_at
		FROM deliveries
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR event_id = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := s.pool.Query(ctx, q, status, eventID, limit, offset)
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

func (s *postgresStore) StatusStats(ctx context.Context) (map[Status]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM deliveries GROUP BY status`)
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
