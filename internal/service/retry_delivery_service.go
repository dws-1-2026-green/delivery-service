package service

import (
	"context"
	"log"
	"time"

	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/model"
)

type RetryDeliveryService struct {
	HttpClient *client.HTTPClient
	MaxRetries int
	Delay      time.Duration
}

func NewRetryDeliveryService(httpClient *client.HTTPClient) *RetryDeliveryService {
	return &RetryDeliveryService{
		HttpClient: httpClient,
		MaxRetries: 3,
		Delay:      2 * time.Second,
	}
}

func (s *RetryDeliveryService) Deliver(ctx context.Context, msg model.DeliveryMessage) error {
	var err error

	for attempt := 0; attempt <= s.MaxRetries; attempt++ {
		err = s.HttpClient.Send(ctx, msg.Subscription.Method, msg.Subscription.DestinationURL, msg.Subscription.Headers, msg.Event.Data)
		if err == nil {
			log.Printf("delivery success: %s (attempt %d)\n", msg.DeliveryID, attempt+1)
			return nil
		}

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

	return err
}