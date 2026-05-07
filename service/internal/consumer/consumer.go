package consumer

import (
	"context"
	"log/slog"

	"github.com/jbisss/webhook-manager/delivery-service/internal/processor"
	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader    *kafka.Reader
	processor processor.DeliveryProcessor
	workers   int
}

func New(brokers []string, topic, groupID string, deliveryProcessor processor.DeliveryProcessor, workers int) *KafkaConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,
	})

	return &KafkaConsumer{
		reader:    r,
		processor: deliveryProcessor,
		workers:   workers,
	}
}

func (c *KafkaConsumer) Start(ctx context.Context) error {
	semaphore := make(chan struct{}, c.workers)

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			slog.Error("error reading message from kafka", slog.Any("error", err))
			continue
		}

		semaphore <- struct{}{}
		go func(m kafka.Message) {
			defer func() { <-semaphore }()
			c.processor.Process(ctx, m.Value)
		}(msg)
	}
}
