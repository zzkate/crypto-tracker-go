# Бенчмарки и сравнение производительности

## Go-бенчмарки (`go test -bench`)

Микробенчмарки репозиториев и HTTP-обработчиков лежат в `bench_test.go`.
Требуют запущенный PostgreSQL (`TEST_DATABASE_URL`):

```bash
scripts/pg-local.sh start
export TEST_DATABASE_URL=postgres://crypto:crypto@127.0.0.1:5432/crypto?sslmode=disable

# все бенчмарки
go test -bench=. -benchmem -run=^$ -count=3

# только HTTP-обработчики
go test -bench=HTTP -benchmem -run=^$
```

Что меряется:

| Бенчмарк | Что делает |
|---|---|
| `BenchmarkCreateCoin` | `INSERT … RETURNING id` + чтение |
| `BenchmarkGetCoin` | выборка одной монеты по id |
| `BenchmarkListCoins` | список 200 монет с `LIMIT/OFFSET` |
| `BenchmarkInsertPrice` | одиночная вставка цены (автокоммит) |
| `BenchmarkListLatestPrices` | `LEFT JOIN LATERAL` последних цен |
| `BenchmarkListPrices` | история по монете |
| `BenchmarkHTTPListCoins` | полный HTTP-путь `/api/coins` (роутинг + auth + JSON + БД) |
| `BenchmarkHTTPLatestPrices` | `/api/prices/latest` |
| `BenchmarkHTTPSync` | `/api/sync` — N вставок в одной транзакции |

## Сравнение Go и Node.js

Честное сравнение — на уровне HTTP, с одинаковой БД и данными:

1. **Один и тот же PostgreSQL** (например, локальный через `scripts/pg-local.sh`).
2. **Одинаковые данные**: одинаковый набор монет и цен.
3. **Одинаковые операции**: CRUD монет, `/api/prices/latest`, история, `/api/sync`.
4. **Прогрев и несколько прогонов**: отбросить холодный старт, взять медиану.
5. **Нагрузочный инструмент**: `hey`, `wrk`, `autocannon` — мерить RPS, p50/p95/p99.

Скрипт для нагрузочного теста:

```bash
# hey (если нет): go install github.com/rakyll/hey@latest
scripts/bench-http.sh http://127.0.0.1:3000/api/prices/latest 50 10s
```

Сравнивать стоит не язык сам по себе, а связку «язык + драйвер БД + HTTP-фреймворк»:
в данном проекте это `net/http` + `pgx` (пул соединений, транзакции) против
Node-варианта с его драйвером. Итоговая разница складывается из:

- времени ответа HTTP-обработчика (роутинг, сериализация JSON),
- конкуренции: пул соединений в Go против event-loop в Node,
- стоимости запросов к БД (часто доминирует),
- аллокаций/GC под нагрузкой.

Итоговые цифры логично фиксировать в таблице вида:

| Эндпоинт | Go (req/s) | Go p99 | Node (req/s) | Node p99 |
|---|---|---|---|---|
| `/api/coins` | … | … | … | … |
| `/api/prices/latest` | … | … | … | … |
| `/api/sync` | … | … | … | … |
