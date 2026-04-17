package service

import (
	"context"

	"github.com/jbisss/webhook-manager/delivery-service/internal/model"
)

type DeliveryService interface {
	Deliver(ctx context.Context, msg model.DeliveryMessage) error
}
