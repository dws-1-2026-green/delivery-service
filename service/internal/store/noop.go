package store

import (
	"context"
	"time"
)

type NopStore struct{}

func (NopStore) Create(_ context.Context, _ DeliveryRecord) error { return nil }
func (NopStore) GetPendingLocked(_ context.Context, _ int) ([]DeliveryRecord, error) {
	return nil, nil
}
func (NopStore) Reschedule(_ context.Context, _ string, _ int, _ time.Time, _ string) error {
	return nil
}
func (NopStore) MarkSuccess(_ context.Context, _ string) error             { return nil }
func (NopStore) MarkExhausted(_ context.Context, _ string, _ string) error { return nil }
func (NopStore) CountPending(_ context.Context) (int, error)               { return 0, nil }
func (NopStore) Transact(_ context.Context, fn func(DeliveryStore) error) error {
	return fn(NopStore{})
}
func (NopStore) ListDeliveries(_ context.Context, _ string, _, _ int) ([]DeliveryRecord, error) {
	return nil, nil
}
func (NopStore) StatusStats(_ context.Context) (map[Status]int, error) { return nil, nil }
