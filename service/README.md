# delivery-service

Сервис доставки вебхуков. Читает задачи из Kafka, отправляет HTTP-запросы на адреса подписчиков с логикой повтора и сохраняет результат в PostgreSQL.

## Flow работы

1. Kafka consumer читает сообщение из топика `deliveries.to_send`. Одновременно обрабатывается до 10 сообщений в параллельных горутинах.

2. Сообщение десериализуется в структуру `DeliveryMessage`, которая содержит идентификатор доставки, данные события, и параметры подписки: URL, HTTP-метод и заголовки.

3. В таблицу `deliveries` вставляется запись со статусом `pending`.

4. Сервис делает до 4 попыток доставки (1 основная + 3 ретрая с паузой 2 секунды между ними). На каждой попытке:
   - Отправляется HTTP-запрос с методом из подписки (GET, POST, PUT или PATCH). Для GET тело запроса не передаётся, для остальных — JSON-payload события.
   - Кастомные заголовки из подписки проставляются в запрос. Если среди них нет `Content-Type`, он выставляется в `application/json` автоматически (кроме GET).
   - В БД обновляются счётчик попыток и текст последней ошибки.

5. Если попытка успешна (HTTP 2xx), статус доставки обновляется до `success` и обработка завершается.

6. Если все попытки исчерпаны, статус обновляется до `exhausted`. Ошибка логируется; сообщение из Kafka считается обработанным (DLQ не реализован).

## Структура проекта

```
delivery-service/
├── cmd/main.go                          — точка входа, инициализация
├── db/migrations/
│   └── 000001_init.up.sql              — схема таблицы deliveries
├── internal/
│   ├── client/http_client.go           — HTTP-клиент (GET/POST/PUT/PATCH)
│   ├── config/config.go                — конфигурация из env
│   ├── consumer/consumer.go            — Kafka consumer, пул воркеров
│   ├── metrics/metrics.go              — Prometheus-метрики
│   ├── model/delivery_message.go       — структура сообщения из Kafka
│   ├── processor/delivery_processor.go — десериализация, вызов сервиса
│   ├── service/
│   │   ├── delivery_service.go         — интерфейс DeliveryService
│   │   └── retry_delivery_service.go   — retry-логика + запись в БД
│   └── store/
│       ├── store.go                    — интерфейс DeliveryStore + типы
│       ├── postgres.go                 — PostgreSQL-реализация
│       └── noop.go                     — заглушка (без DATABASE_URL)
└── go.mod
```

## Сообщение из Kafka

Формат топика `deliveries.to_send`:

```json
{
  "delivery_id": "dlv_01H...",
  "event": {
    "id": "evt_01H...",
    "data": { "order_id": "123" }
  },
  "subscription": {
    "id": "sub_01H...",
    "destination_url": "https://example.com/webhooks",
    "method": "POST",
    "headers": { "X-Secret": "token" }
  },
  "mapped_at": "2026-04-28T10:00:00Z",
  "trace_id": "uuid"
}
```

## База данных

Таблица `deliveries` создаётся автоматически при старте сервиса (если задан `DATABASE_URL`).

```sql
CREATE TABLE deliveries (
    id              TEXT PRIMARY KEY,   -- delivery_id
    event_id        TEXT NOT NULL,      -- для аналитики по событию
    subscription_id TEXT NOT NULL,
    destination_url TEXT NOT NULL,
    method          TEXT NOT NULL,
    status          TEXT NOT NULL,      -- pending | success | exhausted
    attempts        INT  NOT NULL,
    last_error      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);
```

**Запрос всех доставок по событию:**
```sql
SELECT id, subscription_id, status, attempts, last_error, updated_at
FROM deliveries
WHERE event_id = 'evt_01H...'
ORDER BY created_at;
```

## Конфигурация

| Переменная       | Default            | Описание                         |
|------------------|--------------------|----------------------------------|
| `KAFKA_BROKERS`  | `localhost:9092`   | Kafka брокеры (через запятую)    |
| `KAFKA_TOPIC`    | `delivery-events`  | Топик для чтения задач           |
| `KAFKA_GROUP_ID` | `delivery-group`   | Consumer group                   |
| `DATABASE_URL`   | —                  | PostgreSQL DSN (опционально)     |
| `METRICS_ADDR`   | `:9095`            | Адрес `/metrics` эндпоинта       |
| `LOG_LEVEL`      | `info`             | debug / info / warn / error      |
| `LOG_FORMAT`     | `plain`            | plain / json                     |

## Метрики (Prometheus)

Эндпоинт: `GET /metrics` (порт `METRICS_ADDR`)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `delivery_messages_received_total` | Counter | Задачи, прочитанные из Kafka |
| `delivery_attempts_total{status}` | Counter | Попытки доставки: `success` / `failure` |
| `delivery_final_status_total{status}` | Counter | Итог: `success` / `exhausted` |
| `delivery_attempt_duration_seconds` | Histogram | Длительность одного HTTP-вызова |
| `delivery_retries_total` | Counter | Кол-во ретраев (без первой попытки) |
