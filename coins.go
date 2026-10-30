package main

import (
	"database/sql"
)

type Coin struct {
	ID            int     `json:"id"`
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	PriceCurrency string  `json:"priceCurrency"`
	LastUpdatedAt *string `json:"lastUpdatedAt"`
	CreatedAt     string  `json:"createdAt"`
}

func scanCoin(row interface{ Scan(...any) error }) (Coin, error) {
	var c Coin
	var lastUpdated sql.NullString
	err := row.Scan(&c.ID, &c.Symbol, &c.Name, &c.PriceCurrency, &lastUpdated, &c.CreatedAt)
	if err != nil {
		return Coin{}, err
	}
	if lastUpdated.Valid {
		c.LastUpdatedAt = &lastUpdated.String
	}
	return c, nil
}

func createCoin(db dbExecer, symbol, name, currency, createdAt string) (Coin, error) {
	var id int
	err := db.QueryRow(
		"INSERT INTO coins (symbol, name, price_currency, created_at) VALUES ($1, $2, $3, $4) RETURNING id",
		symbol, name, currency, createdAt,
	).Scan(&id)
	if err != nil {
		return Coin{}, err
	}
	return getCoin(db, id)
}

func getCoin(db dbExecer, id int) (Coin, error) {
	c, err := scanCoin(db.QueryRow(
		"SELECT id, symbol, name, price_currency, last_updated_at, created_at FROM coins WHERE id = $1",
		id,
	))
	if err == sql.ErrNoRows {
		return Coin{}, ErrNotFound
	}
	return c, err
}

// allCoins возвращает все монеты (используется синхронизацией, без пагинации).
func allCoins(db dbExecer) ([]Coin, error) {
	rows, err := db.Query("SELECT id, symbol, name, price_currency, last_updated_at, created_at FROM coins ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coins := []Coin{}
	for rows.Next() {
		c, err := scanCoin(rows)
		if err != nil {
			return nil, err
		}
		coins = append(coins, c)
	}
	return coins, rows.Err()
}

func listCoins(db dbExecer, limit, offset int) ([]Coin, error) {
	rows, err := db.Query(
		"SELECT id, symbol, name, price_currency, last_updated_at, created_at FROM coins ORDER BY id ASC LIMIT $1 OFFSET $2",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coins := []Coin{}
	for rows.Next() {
		c, err := scanCoin(rows)
		if err != nil {
			return nil, err
		}
		coins = append(coins, c)
	}
	return coins, rows.Err()
}

func countCoins(db dbExecer) (int, error) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM coins").Scan(&n)
	return n, err
}

func updateCoin(db dbExecer, id int, symbol, name *string) (Coin, error) {
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
	if _, err := db.Exec("UPDATE coins SET symbol = $1, name = $2 WHERE id = $3", nextSymbol, nextName, id); err != nil {
		return Coin{}, err
	}
	return getCoin(db, id)
}

func deleteCoin(db dbExecer, id int) (bool, error) {
	res, err := db.Exec("DELETE FROM coins WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func touchCoin(db dbExecer, id int, updatedAt string) error {
	_, err := db.Exec("UPDATE coins SET last_updated_at = $1 WHERE id = $2", updatedAt, id)
	return err
}
