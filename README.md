# Crypto Price Tracker (Go)

Backend для отслеживания криптовалют и агрегации цен из CoinMarketCap.
Go-версия проекта (аналог `crypto-tracker` на Node.js/TypeScript).

## Возможности

- Bearer-аутентификация (`APP_API_KEY`).
- CRUD отслеживаемых монет.
- Актуальные цены всех монет и история цен по монете (фильтры + пагинация).
- Фоновая синхронизация с CoinMarketCap + ручной запуск.
- Таймауты и обработка ошибок внешнего API, ретраи по 5xx/429/сети.
- PostgreSQL через `pgx` — raw SQL, без ORM; пул соединений (параллельные чтения).
- Метрики Prometheus (`/metrics`).
- Swagger UI (`/docs`) и OpenAPI 3.0 (`/openapi.json`).
- Graceful shutdown по SIGINT/SIGTERM.
- Тесты на стандартной библиотеке (`testing` + `httptest`).

## Стек

- Go 1.27, стандартная библиотека `net/http` (роутинг через `http.ServeMux` 1.22+).
- PostgreSQL: `github.com/jackc/pgx/v5` (пул соединений, без ORM).
- HTTP-клиент CoinMarketCap: `net/http` с таймаутом.
- Тесты: `testing` + `net/http/httptest`.
- Swagger UI: статичный HTML на CDN (без npm/Go-зависимостей для UI).

## Запуск

Нужен PostgreSQL. Через Docker:

```bash
cp .env.example .env   # заполнить APP_API_KEY (и CMC_API_KEY для синхронизации)
docker compose up -d db            # поднять только PostgreSQL
go run .                           # http://localhost:3000/docs
```

Или весь стек в Docker:

```bash
docker compose up -d --build
# http://localhost:3000
```

Без Docker/sudo — локальный PostgreSQL из распакованных бинарников:

```bash
scripts/pg-local.sh start          # initdb + сервер + роль/БД crypto
go run .                           # DATABASE_URL по умолчанию подходит
```

Без `CMC_API_KEY` сервис стартует, но синхронизация отключена (`POST /api/sync` → `502`).

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `PORT` | `3000` | Порт HTTP-сервера |
| `APP_API_KEY` | — | Bearer-токен (обязателен) |
| `CMC_API_KEY` | — | Ключ CoinMarketCap |
| `CMC_BASE_URL` | `https://pro-api.coinmarketcap.com` | Базовый URL CMC |
| `CMC_TIMEOUT_MS` | `5000` | Таймаут запроса к CMC |
| `SYNC_INTERVAL_MS` | `60000` | Интервал фоновой синхронизации |
| `PRICE_CURRENCY` | `USD` | Единая валюта цен |
| `DATABASE_URL` | `postgres://crypto:crypto@localhost:5432/crypto?sslmode=disable` | Строка подключения к PostgreSQL |
| `CMC_BATCH_SIZE` | `100` | Символов в одном запросе |
| `CMC_MAX_RETRIES` | `2` | Повторов при ошибке |
| `CMC_RETRY_BACKOFF_MS` | `200` | База задержки между повторами |

## API

Все `/api/*` требуют `Authorization: Bearer <APP_API_KEY>`.

| Метод | Путь | Описание |
|---|---|---|
| GET | `/health` | Живость |
| GET | `/metrics` | Метрики Prometheus |
| GET | `/openapi.json` | OpenAPI-спецификация |
| GET | `/docs` | Swagger UI |
| GET | `/api/coins` | Список монет (`limit`, `offset`, `X-Total-Count`) |
| POST | `/api/coins` | Добавить монету `{symbol,name}` |
| GET | `/api/coins/{id}` | Монета по id |
| PATCH | `/api/coins/{id}` | Обновить `symbol`/`name` (идемпотентно, `X-Changed`) |
| DELETE | `/api/coins/{id}` | Удалить |
| GET | `/api/prices/latest` | Последние цены всех монет (`limit`, `offset`) |
| GET | `/api/coins/{id}/prices` | История цен (`from`, `to`, `limit`, `offset`) |
| GET | `/api/coins/{id}/prices/latest` | Последняя цена монеты |
| POST | `/api/sync` | Ручная синхронизация |

Коды ошибок: `400` валидация, `401` нет токена, `404` не найдено, `409` дубликат/синк в процессе, `413` большое тело, `502` ошибка CMC.

## Тесты

Тесты с БД требуют запущенный PostgreSQL (переменная `TEST_DATABASE_URL`; без неё они пропускаются).

```bash
# локально: поднять БД и прогнать тесты
scripts/pg-local.sh start
export TEST_DATABASE_URL=postgres://crypto:crypto@127.0.0.1:5432/crypto?sslmode=disable
go test ./...
go test -race ./...
```

В CI тесты гоняются с сервисом `postgres:16-alpine`.
