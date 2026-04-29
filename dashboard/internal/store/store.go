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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Store interface {
	ListDeliveries(ctx context.Context, status, eventID string, limit, offset int) ([]DeliveryRecord, error)
	StatusStats(ctx context.Context) (map[Status]int, error)
}
