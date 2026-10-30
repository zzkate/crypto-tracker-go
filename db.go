package main

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const schema = `
CREATE TABLE IF NOT EXISTS coins (
  id BIGSERIAL PRIMARY KEY,
  symbol TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  price_currency TEXT NOT NULL,
  last_updated_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS price_history (
  id BIGSERIAL PRIMARY KEY,
  coin_id BIGINT NOT NULL REFERENCES coins(id) ON DELETE CASCADE,
  price DOUBLE PRECISION NOT NULL,
  currency TEXT NOT NULL,
  fetched_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_price_history_coin_time
  ON price_history(coin_id, fetched_at DESC, id DESC);
`

// dbExecer — общий интерфейс для *sql.DB и *sql.Tx (репозитории работают и так, и так).
type dbExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// openDB открывает пул соединений к PostgreSQL и применяет схему.
func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	// Пул соединений: PostgreSQL хорошо держит много конкурентных соединений,
	// поэтому чтения идут параллельно (в отличие от одного соединения в SQLite).
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// openDBWithRetry повторяет подключение несколько раз — Postgres может
// стартовать дольше приложения (compose/CI).
func openDBWithRetry(dsn string, attempts int, delay time.Duration) (*sql.DB, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		db, err := openDB(dsn)
		if err == nil {
			return db, nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	return nil, lastErr
}
