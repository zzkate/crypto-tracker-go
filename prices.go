package main

import (
	"errors"

	"gorm.io/gorm"
)

func insertPrice(db *gorm.DB, coinID int64, price float64, currency, fetchedAt string) (PricePoint, error) {
	fetchedAt = normTime(fetchedAt)
	p := PricePoint{CoinID: coinID, Price: price, Currency: currency, FetchedAt: fetchedAt}
	if err := db.Create(&p).Error; err != nil {
		return PricePoint{}, err
	}
	return p, nil
}

func listPrices(db *gorm.DB, coinID int64, from, to string, limit, offset int) ([]PricePoint, error) {
	q := db.Where("coin_id = ?", coinID)
	if from != "" {
		q = q.Where("fetched_at >= ?", from)
	}
	if to != "" {
		q = q.Where("fetched_at <= ?", to)
	}
	points := make([]PricePoint, 0)
	if err := q.Order("fetched_at DESC, id DESC").Limit(limit).Offset(offset).Find(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

func countPrices(db *gorm.DB, coinID int64, from, to string) (int, error) {
	q := db.Model(&PricePoint{}).Where("coin_id = ?", coinID)
	if from != "" {
		q = q.Where("fetched_at >= ?", from)
	}
	if to != "" {
		q = q.Where("fetched_at <= ?", to)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

func getLatestPrice(db *gorm.DB, coinID int64) (PricePoint, error) {
	var p PricePoint
	if err := db.Where("coin_id = ?", coinID).Order("fetched_at DESC, id DESC").First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PricePoint{}, ErrNotFound
		}
		return PricePoint{}, err
	}
	return p, nil
}

func listLatestPrices(db *gorm.DB, limit, offset int) ([]LatestPrice, error) {
	// Каждый подзапрос — index-scan по (coin_id, fetched_at DESC, id DESC).
	latest := make([]LatestPrice, 0)
	err := db.Raw(`
		SELECT c.id AS coin_id, c.symbol, c.name,
		       COALESCE(p.currency, c.price_currency) AS currency,
		       p.price, p.fetched_at
		FROM coins c
		LEFT JOIN price_history p ON p.id = (
		   SELECT id FROM price_history
		   WHERE coin_id = c.id
		   ORDER BY fetched_at DESC, id DESC
		   LIMIT 1
		)
		ORDER BY c.id ASC LIMIT ? OFFSET ?`, limit, offset).Scan(&latest).Error
	if err != nil {
		return nil, err
	}
	return latest, nil
}
