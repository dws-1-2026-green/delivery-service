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
	Headers        map[string]string
	Payload        []byte
	Status         Status
	Attempts       int
	NextAttempt    *time.Time
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DeliveryStore interface {
	Create(ctx context.Context, r DeliveryRecord) error
	GetPendingLocked(ctx context.Context, limit int) ([]DeliveryRecord, error)
	Reschedule(ctx context.Context, id string, attempts int, nextAttempt time.Time, lastError string) error
	MarkSuccess(ctx context.Context, id string) error
	MarkExhausted(ctx context.Context, id string, lastError string) error
	CountPending(ctx context.Context) (int, error)
	Transact(ctx context.Context, fn func(DeliveryStore) error) error

	ListDeliveries(ctx context.Context, status string, limit, offset int) ([]DeliveryRecord, error)
	StatusStats(ctx context.Context) (map[Status]int, error)
}
