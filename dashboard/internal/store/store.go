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

type Store interface {
	ListDeliveries(ctx context.Context, status, eventID, subscriptionID, destinationURL string, limit, offset int) ([]DeliveryRecord, error)
	GroupDeliveries(ctx context.Context, field, status, eventID, subscriptionID, destinationURL string) ([]GroupRow, error)
	StatusStats(ctx context.Context) (map[Status]int, error)
}
