package main

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openDB открывает пул соединений GORM к PostgreSQL, применяет схему (AutoMigrate)
// и Postgres-специфичные объекты (FK с каскадом, составной индекс).
func openDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Coin{}, &PricePoint{}); err != nil {
		return nil, err
	}

	// FK с каскадным удалением (GORM сам его не создаёт без ассоциаций).
	if err := db.Exec(`ALTER TABLE price_history DROP CONSTRAINT IF EXISTS fk_price_history_coin`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE price_history ADD CONSTRAINT fk_price_history_coin FOREIGN KEY (coin_id) REFERENCES coins(id) ON DELETE CASCADE`).Error; err != nil {
		return nil, err
	}
	// Составной индекс для выборок по (coin_id, fetched_at DESC, id DESC).
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_price_history_coin_time ON price_history(coin_id, fetched_at DESC, id DESC)`).Error; err != nil {
		return nil, err
	}

	// Пул соединений: PostgreSQL держит много конкурентных соединений.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

// openDBWithRetry повторяет подключение несколько раз — Postgres может
// стартовать дольше приложения (compose/CI).
func openDBWithRetry(dsn string, attempts int, delay time.Duration) (*gorm.DB, error) {
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

// closeDB закрывает нижележащий пул database/sql.
func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
