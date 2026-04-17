package model

import (
	"encoding/json"
	"time"
)

type DeliveryMessage struct {
	DeliveryID   string       `json:"delivery_id"` // id доставки
	Event        Event        `json:"event"`
	Subscription Subscription `json:"subscription"`
	MappedAt     time.Time    `json:"mapped_at"` // ISO-8601 Время создания задачи
	TraceID      string       `json:"trace_id"`  // trace-id из сервиса получения событий
}

type Event struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

type Subscription struct {
	ID             string            `json:"id"` // id подписки в сервисе подписок
	DestinationURL string            `json:"destination_url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
}
