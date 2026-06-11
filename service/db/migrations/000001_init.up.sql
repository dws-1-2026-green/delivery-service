CREATE TABLE IF NOT EXISTS deliveries (
    id              TEXT        PRIMARY KEY,
    event_id        TEXT        NOT NULL,
    subscription_id TEXT        NOT NULL,
    destination_url TEXT        NOT NULL,
    method          TEXT        NOT NULL,
    headers         JSONB       NOT NULL DEFAULT '{}',
    payload         BYTEA       NOT NULL DEFAULT '',
    status          TEXT        NOT NULL DEFAULT 'pending',
    attempts        INT         NOT NULL DEFAULT 0,
    next_attempt    TIMESTAMPTZ,
    last_error      TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE deliveries ADD COLUMN IF NOT EXISTS headers      JSONB       NOT NULL DEFAULT '{}';
ALTER TABLE deliveries ADD COLUMN IF NOT EXISTS payload      BYTEA       NOT NULL DEFAULT '';
ALTER TABLE deliveries ADD COLUMN IF NOT EXISTS next_attempt TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_deliveries_event_id ON deliveries (event_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_pending  ON deliveries (next_attempt) WHERE status = 'pending';
