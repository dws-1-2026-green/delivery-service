package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
	"github.com/jbisss/webhook-manager/delivery-service/internal/model"
)

type RetryDeliveryService struct {
	HttpClient *client.HTTPClient
	MaxRetries int
	Delay      time.Duration
}

func NewRetryDeliveryService(client *client.HTTPClient) *RetryDeliveryService {
	return &RetryDeliveryService{
		HttpClient: client,
		MaxRetries: 3,
		Delay:      2 * time.Second,
	}
}

func (s *RetryDeliveryService) Deliver(ctx context.Context, msg model.DeliveryMessage) error {
	var err error

	for attempt := 0; attempt <= s.MaxRetries; attempt++ {
		if attempt > 0 {
			metrics.DeliveryRetriesTotal.Inc()
		}

		start := time.Now()
		err = s.HttpClient.Send(ctx, msg.Subscription.Method, msg.Subscription.DestinationURL, msg.Subscription.Headers, msg.Event.Data)
		metrics.DeliveryAttemptDuration.Observe(time.Since(start).Seconds())

		if err == nil {
			metrics.DeliveryAttempts.WithLabelValues("success").Inc()
			metrics.DeliveryFinalStatus.WithLabelValues("success").Inc()
			slog.Info("delivery success",
				slog.String("delivery_id", msg.DeliveryID),
				slog.Int("attempt", attempt+1),
			)
			return nil
		}

		metrics.DeliveryAttempts.WithLabelValues("failure").Inc()
		slog.Warn("delivery attempt failed",
			slog.String("delivery_id", msg.DeliveryID),
			slog.Int("attempt", attempt+1),
			slog.Any("error", err),
		)

		if attempt == s.MaxRetries {
			break
		}

		select {
		case <-time.After(s.Delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	metrics.DeliveryFinalStatus.WithLabelValues("exhausted").Inc()
	return err
}
