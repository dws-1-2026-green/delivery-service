CREATE TABLE IF NOT EXISTS deliveries (
    id              TEXT PRIMARY KEY,           -- delivery_id из сообщения Kafka
    event_id        TEXT NOT NULL,              -- event.id — ключ для аналитики по событию
    subscription_id TEXT NOT NULL,              -- subscription.id
    destination_url TEXT NOT NULL,
    method          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending', -- pending | success | exhausted
    attempts        INT  NOT NULL DEFAULT 0,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deliveries_event_id ON deliveries (event_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_status   ON deliveries (status);
