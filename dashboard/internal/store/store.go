package store

import (
	"context"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusSuccess   Status = "success"
	StatusExhausted Status = "exhausted"
)

type DeliveryRecord struct {
	ID             string
	EventID        string
	SubscriptionID string
	DestinationURL string
	Method         string
	Status         Status
	Attempts       int
	NextAttempt    *time.Time
	LastError      string
	Payload        []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type GroupRow struct {
	Value     string
	Total     int
	Success   int
	Pending   int
	Exhausted int
}

type RetryBucket struct {
	Label    string
	Count    int
	Pct      int
	BarClass string
}

type ThroughputPoint struct {
	Minute time.Time
	Count  int
}

type Store interface {
	ListDeliveries(ctx context.Context, status, eventID, subscriptionID, destinationURL, attemptsOp string, attemptsVal, limit, offset int) ([]DeliveryRecord, error)
	GroupDeliveries(ctx context.Context, field, status, eventID, subscriptionID, destinationURL string) ([]GroupRow, error)
	StatusStats(ctx context.Context) (map[Status]int, error)
	FilteredStats(ctx context.Context, status, eventID, subscriptionID, destinationURL, attemptsOp string, attemptsVal int) (map[Status]int, error)
	RetryDistribution(ctx context.Context) ([]RetryBucket, error)
	ThroughputSeries(ctx context.Context) ([]ThroughputPoint, error)
}
