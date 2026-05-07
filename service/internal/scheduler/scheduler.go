package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jbisss/webhook-manager/delivery-service/internal/backoff"
	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
	"github.com/jbisss/webhook-manager/delivery-service/internal/store"
)

type Scheduler struct {
	store     store.DeliveryStore
	client    *client.HTTPClient
	backoff   backoff.Config
	semaphore chan struct{}
}

func New(s store.DeliveryStore, c *client.HTTPClient, cfg backoff.Config, workers int) *Scheduler {
	return &Scheduler{
		store:     s,
		client:    c,
		backoff:   cfg,
		semaphore: make(chan struct{}, workers),
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.backoff.BaseDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	start := time.Now()
	defer func() {
		metrics.SchedulerTickDuration.Observe(time.Since(start).Seconds())
	}()

	var deliveries []store.DeliveryRecord

	err := s.store.Transact(ctx, func(tx store.DeliveryStore) error {
		var err error
		deliveries, err = tx.GetPendingLocked(ctx, cap(s.semaphore))
		if err != nil {
			return err
		}

		now := time.Now()
		for _, d := range deliveries {
			if d.Attempts >= s.backoff.MaxAttempts {
				continue
			}
			nextAttempt := now.Add(s.backoff.Delay(d.Attempts))
			if err := tx.Reschedule(ctx, d.ID, d.Attempts+1, nextAttempt, d.LastError); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("scheduler tick transaction failed", "error", err)
		return
	}

	if len(deliveries) > 0 {
		slog.Info("scheduler tick", "pending", len(deliveries))
	}

	var wg sync.WaitGroup
	for _, d := range deliveries {
		s.semaphore <- struct{}{}
		wg.Add(1)
		go func(delivery store.DeliveryRecord) {
			defer wg.Done()
			defer func() { <-s.semaphore }()
			s.attempt(ctx, delivery)
		}(d)
	}
	wg.Wait()

	if count, err := s.store.CountPending(ctx); err == nil {
		metrics.PendingDeliveries.Set(float64(count))
	} else {
		slog.Error("failed to count pending deliveries", "error", err)
	}
}

func (s *Scheduler) attempt(ctx context.Context, d store.DeliveryRecord) {
	err := s.client.Send(ctx, d.Method, d.DestinationURL, d.Headers, d.Payload)

	metrics.DeliveryRetriesTotal.Inc()

	if err == nil {
		if markErr := s.store.MarkSuccess(ctx, d.ID); markErr != nil {
			slog.Error("failed to mark success", "delivery_id", d.ID, "error", markErr)
			return
		}
		slog.Info("delivery succeeded", "delivery_id", d.ID, "attempt", d.Attempts)
		metrics.DeliveryAttempts.WithLabelValues("success").Inc()
		metrics.DeliveryFinalStatus.WithLabelValues("success").Inc()
		return
	}

	slog.Warn("delivery attempt failed", "delivery_id", d.ID, "attempt", d.Attempts, "error", err)
	metrics.DeliveryAttempts.WithLabelValues("failure").Inc()

	var httpErr *client.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
		if markErr := s.store.MarkExhausted(ctx, d.ID, err.Error()); markErr != nil {
			slog.Error("failed to mark exhausted (4xx)", "delivery_id", d.ID, "error", markErr)
			return
		}
		slog.Error("delivery exhausted (4xx fast-fail)", "delivery_id", d.ID, "status", httpErr.StatusCode)
		metrics.DeliveryFinalStatus.WithLabelValues("exhausted").Inc()
		return
	}

	if d.Attempts >= s.backoff.MaxAttempts {
		if markErr := s.store.MarkExhausted(ctx, d.ID, err.Error()); markErr != nil {
			slog.Error("failed to mark exhausted", "delivery_id", d.ID, "error", markErr)
			return
		}
		slog.Error("delivery exhausted (max attempts)", "delivery_id", d.ID, "attempts", d.Attempts)
		metrics.DeliveryFinalStatus.WithLabelValues("exhausted").Inc()
	}
	// else: next attempt already pre-scheduled in DB by tick()
}
