package service

import (
	"context"
	"log"
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
		err = s.HttpClient.Send(ctx, msg.Subscription.DestinationURL, msg.Event.Data)
		metrics.DeliveryAttemptDuration.Observe(time.Since(start).Seconds())

		if err == nil {
			metrics.DeliveryAttempts.WithLabelValues("success").Inc()
			metrics.DeliveryFinalStatus.WithLabelValues("success").Inc()
			log.Printf("delivery success: %s (attempt %d)\n", msg.DeliveryID, attempt+1)
			return nil
		}

		metrics.DeliveryAttempts.WithLabelValues("failure").Inc()
		log.Printf("delivery failed: %s (attempt %d): %v\n", msg.DeliveryID, attempt+1, err)

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
