-- Схема
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

CREATE INDEX IF NOT EXISTS idx_deliveries_event_id ON deliveries (event_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_pending  ON deliveries (next_attempt) WHERE status = 'pending';

-- Успешные доставки
INSERT INTO deliveries (id, event_id, subscription_id, destination_url, method, headers, payload, status, attempts, next_attempt, last_error, created_at, updated_at) VALUES
(
    'dlv_01HXSAMPLE001',
    'evt_01HX111AAA',
    'sub_01HX_SUB_A',
    'https://crm.example.com/webhooks/orders',
    'POST',
    '{"X-Secret": "tok_crm_abc", "Content-Type": "application/json"}',
    '{"order_id":"123","amount":1990}',
    'success',
    1,
    NULL,
    '',
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '2 hours'
),
(
    'dlv_01HXSAMPLE002',
    'evt_01HX111BBB',
    'sub_01HX_SUB_A',
    'https://crm.example.com/webhooks/orders',
    'POST',
    '{"X-Secret": "tok_crm_abc", "Content-Type": "application/json"}',
    '{"order_id":"124","amount":5500}',
    'success',
    1,
    NULL,
    '',
    NOW() - INTERVAL '90 minutes',
    NOW() - INTERVAL '90 minutes'
),
(
    'dlv_01HXSAMPLE003',
    'evt_01HX222CCC',
    'sub_01HX_SUB_B',
    'https://analytics.example.com/ingest',
    'POST',
    '{"Authorization": "Bearer eyJhbGc..."}',
    '{"user_id":"u-77","action":"login"}',
    'success',
    2,
    NULL,
    'dial tcp: connection refused',
    NOW() - INTERVAL '1 hour',
    NOW() - INTERVAL '55 minutes'
),
(
    'dlv_01HXSAMPLE004',
    'evt_01HX333DDD',
    'sub_01HX_SUB_C',
    'https://shop.partner.io/events',
    'PUT',
    '{}',
    '{"sku":"PROD-42","qty":3}',
    'success',
    1,
    NULL,
    '',
    NOW() - INTERVAL '45 minutes',
    NOW() - INTERVAL '45 minutes'
),

-- Ожидают доставки (next_attempt в прошлом — прямо сейчас должны обрабатываться)
(
    'dlv_01HXSAMPLE005',
    'evt_01HX444EEE',
    'sub_01HX_SUB_D',
    'https://notify.internal/hook',
    'POST',
    '{"X-Hub-Signature": "sha256=abc123"}',
    '{"payment_id":"pay-99","status":"completed"}',
    'pending',
    0,
    NOW() - INTERVAL '5 seconds',
    '',
    NOW() - INTERVAL '1 minute',
    NOW() - INTERVAL '1 minute'
),
(
    'dlv_01HXSAMPLE006',
    'evt_01HX444FFF',
    'sub_01HX_SUB_D',
    'https://notify.internal/hook',
    'POST',
    '{"X-Hub-Signature": "sha256=def456"}',
    '{"payment_id":"pay-100","status":"refunded"}',
    'pending',
    1,
    NOW() - INTERVAL '2 seconds',
    'context deadline exceeded (Client.Timeout exceeded while awaiting headers)',
    NOW() - INTERVAL '3 minutes',
    NOW() - INTERVAL '1 minute'
),

-- Ожидают доставки с будущим next_attempt (запланированы на ретрай)
(
    'dlv_01HXSAMPLE007',
    'evt_01HX555GGG',
    'sub_01HX_SUB_E',
    'https://legacy.corp.local/receiver',
    'POST',
    '{}',
    '{"event":"user.deleted","user_id":"u-99"}',
    'pending',
    2,
    NOW() + INTERVAL '18 seconds',
    'HTTP 503: Service Unavailable',
    NOW() - INTERVAL '10 minutes',
    NOW() - INTERVAL '2 minutes'
),
(
    'dlv_01HXSAMPLE008',
    'evt_01HX555HHH',
    'sub_01HX_SUB_F',
    'https://slack-compat.example.com/hooks/T012/B034/xyz',
    'POST',
    '{"Content-Type": "application/json"}',
    '{"text":"Order shipped: #125"}',
    'pending',
    1,
    NOW() + INTERVAL '45 seconds',
    'HTTP 429: Too Many Requests',
    NOW() - INTERVAL '5 minutes',
    NOW() - INTERVAL '3 minutes'
),

-- Исчерпанные (failed after all retries)
(
    'dlv_01HXSAMPLE009',
    'evt_01HX666III',
    'sub_01HX_SUB_G',
    'https://dead.endpoint.example.com/webhooks',
    'POST',
    '{"X-API-Key": "key_prod_000"}',
    '{"invoice_id":"inv-55","total":8800}',
    'exhausted',
    3,
    NULL,
    'HTTP 404: Not Found — endpoint decommissioned',
    NOW() - INTERVAL '30 minutes',
    NOW() - INTERVAL '10 minutes'
),
(
    'dlv_01HXSAMPLE010',
    'evt_01HX666JJJ',
    'sub_01HX_SUB_G',
    'https://dead.endpoint.example.com/webhooks',
    'POST',
    '{"X-API-Key": "key_prod_000"}',
    '{"invoice_id":"inv-56","total":450}',
    'exhausted',
    3,
    NULL,
    'HTTP 404: Not Found — endpoint decommissioned',
    NOW() - INTERVAL '25 minutes',
    NOW() - INTERVAL '5 minutes'
),
(
    'dlv_01HXSAMPLE011',
    'evt_01HX777KKK',
    'sub_01HX_SUB_H',
    'https://partner-api.biz/hook',
    'POST',
    '{"Authorization": "Basic dXNlcjpwYXNz"}',
    '{"order_id":"999","status":"cancelled"}',
    'exhausted',
    3,
    NULL,
    'dial tcp 93.184.216.34:443: i/o timeout',
    NOW() - INTERVAL '1 hour',
    NOW() - INTERVAL '20 minutes'
),

-- Свежие — только что добавились в очередь
(
    'dlv_01HXSAMPLE012',
    'evt_01HX888LLL',
    'sub_01HX_SUB_A',
    'https://crm.example.com/webhooks/orders',
    'POST',
    '{"X-Secret": "tok_crm_abc", "Content-Type": "application/json"}',
    '{"order_id":"200","amount":320}',
    'pending',
    0,
    NOW(),
    '',
    NOW() - INTERVAL '10 seconds',
    NOW() - INTERVAL '10 seconds'
);
