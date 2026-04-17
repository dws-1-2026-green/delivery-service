package consumer

import (
	"context"
	"log"

	"github.com/jbisss/webhook-manager/delivery-service/internal/processor"
	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader    *kafka.Reader
	processor processor.DeliveryProcessor
}

func New(brokers []string, topic, groupID string, deliveryProcessor processor.DeliveryProcessor) *KafkaConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,
	})

	return &KafkaConsumer{
		reader:    r,
		processor: deliveryProcessor,
	}
}

func (c *KafkaConsumer) Start(ctx context.Context) error {
	const maxWorkers = 10
	semaphore := make(chan struct{}, maxWorkers)

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			log.Println("error reading message:", err)
			continue
		}

		semaphore <- struct{}{}
		go func(m kafka.Message) {
			defer func() { <-semaphore }()
			c.processor.Process(ctx, m.Value)
		}(msg)
	}
}
