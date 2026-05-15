package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}

func (s *postgresStore) ListDeliveries(ctx context.Context, status, eventID, subscriptionID, destinationURL string, limit, offset int) ([]DeliveryRecord, error) {
	const q = `
		SELECT id, event_id, subscription_id, destination_url, method, status,
		       attempts, next_attempt, last_error, payload, created_at, updated_at
		FROM deliveries
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR event_id = $2)
		  AND ($3 = '' OR subscription_id = $3)
		  AND ($4 = '' OR destination_url = $4)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6
	`
	rows, err := s.pool.Query(ctx, q, status, eventID, subscriptionID, destinationURL, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []DeliveryRecord
	for rows.Next() {
		var r DeliveryRecord
		if err := rows.Scan(
			&r.ID, &r.EventID, &r.SubscriptionID, &r.DestinationURL, &r.Method, &r.Status,
			&r.Attempts, &r.NextAttempt, &r.LastError, &r.Payload, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *postgresStore) GroupDeliveries(ctx context.Context, field, status, eventID, subscriptionID, destinationURL string) ([]GroupRow, error) {
	var col string
	switch field {
	case "event_id":
		col = "event_id"
	case "subscription_id":
		col = "subscription_id"
	case "destination_url":
		col = "destination_url"
	default:
		return nil, fmt.Errorf("invalid group field: %q", field)
	}

	q := fmt.Sprintf(`
		SELECT %s,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status = 'success'   THEN 1 ELSE 0 END) AS success,
		       SUM(CASE WHEN status = 'pending'   THEN 1 ELSE 0 END) AS pending,
		       SUM(CASE WHEN status = 'exhausted' THEN 1 ELSE 0 END) AS exhausted
		FROM deliveries
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR event_id = $2)
		  AND ($3 = '' OR subscription_id = $3)
		  AND ($4 = '' OR destination_url = $4)
		GROUP BY %s
		ORDER BY total DESC
	`, col, col)

	rows, err := s.pool.Query(ctx, q, status, eventID, subscriptionID, destinationURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GroupRow
	for rows.Next() {
		var gr GroupRow
		if err := rows.Scan(&gr.Value, &gr.Total, &gr.Success, &gr.Pending, &gr.Exhausted); err != nil {
			return nil, err
		}
		result = append(result, gr)
	}
	return result, rows.Err()
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
