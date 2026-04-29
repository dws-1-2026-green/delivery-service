package processor

import (
	"context"
)

type DeliveryProcessor interface {
	Process(ctx context.Context, msgRawBytes []byte)
}
