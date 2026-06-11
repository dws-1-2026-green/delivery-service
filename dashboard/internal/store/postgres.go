package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}

func (s *postgresStore) ListDeliveries(ctx context.Context, status, eventID, subscriptionID, destinationURL, attemptsOp string, attemptsVal, limit, offset int) ([]DeliveryRecord, error) {
	attemptsClause := ""
	if attemptsOp == ">" || attemptsOp == "<" || attemptsOp == "=" {
		attemptsClause = fmt.Sprintf("AND attempts %s %d", attemptsOp, attemptsVal)
	}
	q := fmt.Sprintf(`
		SELECT id, event_id, subscription_id, destination_url, method, status,
		       attempts, next_attempt, last_error, payload, created_at, updated_at
		FROM deliveries
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR event_id = $2)
		  AND ($3 = '' OR subscription_id = $3)
		  AND ($4 = '' OR destination_url = $4)
		  %s
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6
	`, attemptsClause)
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

func (s *postgresStore) FilteredStats(ctx context.Context, status, eventID, subscriptionID, destinationURL, attemptsOp string, attemptsVal int) (map[Status]int, error) {
	attemptsClause := ""
	if attemptsOp == ">" || attemptsOp == "<" || attemptsOp == "=" {
		attemptsClause = fmt.Sprintf("AND attempts %s %d", attemptsOp, attemptsVal)
	}
	q := fmt.Sprintf(`
		SELECT status, COUNT(*)
		FROM deliveries
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR event_id = $2)
		  AND ($3 = '' OR subscription_id = $3)
		  AND ($4 = '' OR destination_url = $4)
		  %s
		GROUP BY status
	`, attemptsClause)

	rows, err := s.pool.Query(ctx, q, status, eventID, subscriptionID, destinationURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[Status]int{StatusPending: 0, StatusSuccess: 0, StatusExhausted: 0}
	for rows.Next() {
		var s Status
		var cnt int
		if err := rows.Scan(&s, &cnt); err != nil {
			return nil, err
		}
		stats[s] = cnt
	}
	return stats, rows.Err()
}

func (s *postgresStore) ThroughputSeries(ctx context.Context) ([]ThroughputPoint, error) {
	const q = `
		SELECT DATE_TRUNC('minute', created_at) AS minute, COUNT(*) AS cnt
		FROM deliveries
		WHERE created_at > NOW() - INTERVAL '60 minutes'
		GROUP BY minute
		ORDER BY minute
	`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make(map[time.Time]int)
	for rows.Next() {
		var minute time.Time
		var cnt int
		if err := rows.Scan(&minute, &cnt); err != nil {
			return nil, err
		}
		data[minute.UTC().Truncate(time.Minute)] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Truncate(time.Minute)
	result := make([]ThroughputPoint, 60)
	for i := 0; i < 60; i++ {
		t := now.Add(time.Duration(i-59) * time.Minute)
		result[i] = ThroughputPoint{Minute: t, Count: data[t]}
	}
	return result, nil
}

func (s *postgresStore) RetryDistribution(ctx context.Context) ([]RetryBucket, error) {
	const q = `
		SELECT LEAST(attempts, 3) AS bucket, COUNT(*) AS cnt
		FROM deliveries
		GROUP BY bucket
		ORDER BY bucket
	`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[int]int{0: 0, 1: 0, 2: 0, 3: 0}
	for rows.Next() {
		var bucket, cnt int
		if err := rows.Scan(&bucket, &cnt); err != nil {
			return nil, err
		}
		counts[bucket] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	maxCount := 1
	for _, v := range counts {
		if v > maxCount {
			maxCount = v
		}
	}

	labels := []string{"0", "1", "2", "3+"}
	classes := []string{"rc-0", "rc-1", "rc-2", "rc-3p"}
	result := make([]RetryBucket, 4)
	for i := 0; i < 4; i++ {
		cnt := counts[i]
		result[i] = RetryBucket{
			Label:    labels[i],
			Count:    cnt,
			Pct:      cnt * 100 / maxCount,
			BarClass: classes[i],
		}
	}
	return result, nil
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
