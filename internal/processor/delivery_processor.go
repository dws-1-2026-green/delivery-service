package processor

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
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
	metrics.MessagesReceived.Inc()

	var msg model.DeliveryMessage
	if err := json.Unmarshal(msgRawBytes, &msg); err != nil {
		slog.Error("failed to unmarshal delivery message", slog.Any("error", err))
		return
	}

	if err := p.Service.Deliver(ctx, msg); err != nil {
		slog.Error("failed to deliver message",
			slog.String("delivery_id", msg.DeliveryID),
			slog.Any("error", err),
		)
		// можно добавить DLQ или логирование в БД
	}
}
