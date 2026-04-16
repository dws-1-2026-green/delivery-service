package processor

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jbisss/webhook-manager/delivery-service/internal/model"
	"github.com/jbisss/webhook-manager/delivery-service/internal/service"
)

type SimpleDeliveryProcessor struct {
	Service service.DeliveryService
}

func New(service service.DeliveryService) *SimpleDeliveryProcessor {
	return &SimpleDeliveryProcessor{
		Service: service,
	}
}

func (p *SimpleDeliveryProcessor) Process(ctx context.Context, msgRawBytes []byte) {
	var msg model.DeliveryMessage

	if err := json.Unmarshal(msgRawBytes, &msg); err != nil {
		log.Printf("Failed to unmarshal message: %v\n", err)
		return
	}

	err := p.Service.Deliver(ctx, msg)
	if err != nil {
		log.Printf("Failed to deliver message %s: %v\n", msg.DeliveryID, err)
		// можно добавить DLQ или логирование в БД
	}
}
