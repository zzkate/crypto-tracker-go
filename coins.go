package main

import (
	"errors"

	"gorm.io/gorm"
)

func createCoin(db *gorm.DB, symbol, name, currency, createdAt string) (Coin, error) {
	coin := Coin{Symbol: symbol, Name: name, PriceCurrency: currency, CreatedAt: createdAt}
	if err := db.Create(&coin).Error; err != nil {
		return Coin{}, err
	}
	return coin, nil
}

func getCoin(db *gorm.DB, id int64) (Coin, error) {
	var c Coin
	if err := db.First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Coin{}, ErrNotFound
		}
		return Coin{}, err
	}
	return c, nil
}

// allCoins возвращает все монеты (используется синхронизацией, без пагинации).
func allCoins(db *gorm.DB) ([]Coin, error) {
	coins := make([]Coin, 0)
	if err := db.Order("id ASC").Find(&coins).Error; err != nil {
		return nil, err
	}
	return coins, nil
}

func listCoins(db *gorm.DB, limit, offset int) ([]Coin, error) {
	coins := make([]Coin, 0)
	if err := db.Order("id ASC").Limit(limit).Offset(offset).Find(&coins).Error; err != nil {
		return nil, err
	}
	return coins, nil
}

func countCoins(db *gorm.DB) (int, error) {
	var n int64
	if err := db.Model(&Coin{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}

func updateCoin(db *gorm.DB, id int64, symbol, name *string) (Coin, error) {
	cur, err := getCoin(db, id)
	if err != nil {
		return Coin{}, err
	}
	nextSymbol, nextName := cur.Symbol, cur.Name
	if symbol != nil {
		nextSymbol = *symbol
	}
	if name != nil {
		nextName = *name
	}
	if err := db.Model(&Coin{}).Where("id = ?", id).Updates(map[string]any{"symbol": nextSymbol, "name": nextName}).Error; err != nil {
		return Coin{}, err
	}
	return getCoin(db, id)
}

func deleteCoin(db *gorm.DB, id int64) (bool, error) {
	res := db.Delete(&Coin{}, id)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func touchCoin(db *gorm.DB, id int64, updatedAt string) error {
	return db.Model(&Coin{}).Where("id = ?", id).Update("last_updated_at", updatedAt).Error
}
