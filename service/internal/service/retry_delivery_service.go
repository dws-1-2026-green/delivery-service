package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jbisss/webhook-manager/delivery-service/internal/backoff"
	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
	"github.com/jbisss/webhook-manager/delivery-service/internal/model"
	"github.com/jbisss/webhook-manager/delivery-service/internal/store"
)

type RetryDeliveryService struct {
	HttpClient *client.HTTPClient
	Store      store.DeliveryStore
	Backoff    backoff.Config
}

func NewRetryDeliveryService(httpClient *client.HTTPClient, deliveryStore store.DeliveryStore, cfg backoff.Config) *RetryDeliveryService {
	return &RetryDeliveryService{
		HttpClient: httpClient,
		Store:      deliveryStore,
		Backoff:    cfg,
	}
}

func (s *RetryDeliveryService) Deliver(ctx context.Context, msg model.DeliveryMessage) error {
	// Save first so the delivery is always tracked regardless of what happens next.
	if err := s.Store.Create(ctx, store.DeliveryRecord{
		ID:             msg.DeliveryID,
		EventID:        msg.Event.ID,
		SubscriptionID: msg.Subscription.ID,
		DestinationURL: msg.Subscription.DestinationURL,
		Method:         msg.Subscription.Method,
		Headers:        msg.Subscription.Headers,
		Payload:        msg.Event.Data,
		Status:         store.StatusPending,
	}); err != nil {
		slog.Warn("failed to create delivery record",
			slog.String("delivery_id", msg.DeliveryID),
			slog.Any("error", err),
		)
	}

	start := time.Now()
	err := s.HttpClient.Send(ctx, msg.Subscription.Method, msg.Subscription.DestinationURL, msg.Subscription.Headers, msg.Event.Data)
	metrics.DeliveryAttemptDuration.Observe(time.Since(start).Seconds())

	if err == nil {
		metrics.DeliveryAttempts.WithLabelValues("success").Inc()
		metrics.DeliveryFinalStatus.WithLabelValues("success").Inc()
		slog.Info("delivery succeeded on first attempt", slog.String("delivery_id", msg.DeliveryID))
		if dbErr := s.Store.MarkSuccess(ctx, msg.DeliveryID); dbErr != nil {
			slog.Warn("failed to mark delivery success", slog.String("delivery_id", msg.DeliveryID), slog.Any("error", dbErr))
		}
		return nil
	}

	metrics.DeliveryAttempts.WithLabelValues("failure").Inc()

	var httpErr *client.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
		slog.Error("delivery exhausted (4xx fast-fail)",
			slog.String("delivery_id", msg.DeliveryID),
			slog.Int("status_code", httpErr.StatusCode),
		)
		metrics.DeliveryFinalStatus.WithLabelValues("exhausted").Inc()
		if dbErr := s.Store.MarkExhausted(ctx, msg.DeliveryID, err.Error()); dbErr != nil {
			slog.Warn("failed to mark delivery exhausted", slog.String("delivery_id", msg.DeliveryID), slog.Any("error", dbErr))
		}
		return nil
	}

	nextAttempt := time.Now().Add(s.Backoff.Delay(1))
	slog.Warn("first attempt failed, scheduling retry",
		slog.String("delivery_id", msg.DeliveryID),
		slog.Time("next_attempt", nextAttempt),
		slog.Any("error", err),
	)
	if dbErr := s.Store.Reschedule(ctx, msg.DeliveryID, 1, nextAttempt, err.Error()); dbErr != nil {
		slog.Warn("failed to reschedule delivery", slog.String("delivery_id", msg.DeliveryID), slog.Any("error", dbErr))
	}

	// Return nil so Kafka offset is committed — retries are owned by the scheduler.
	return nil
}
