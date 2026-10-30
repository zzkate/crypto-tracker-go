package main

import (
	"database/sql"
	"fmt"
	"strings"
)

type PricePoint struct {
	ID        int     `json:"id"`
	CoinID    int     `json:"coinId"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	FetchedAt string  `json:"fetchedAt"`
}

type LatestPrice struct {
	CoinID    int      `json:"coinId"`
	Symbol    string   `json:"symbol"`
	Name      string   `json:"name"`
	Price     *float64 `json:"price"`
	Currency  string   `json:"currency"`
	FetchedAt *string  `json:"fetchedAt"`
}

func insertPrice(db dbExecer, coinID int, price float64, currency, fetchedAt string) (PricePoint, error) {
	fetchedAt = normTime(fetchedAt)
	var id int
	err := db.QueryRow(
		"INSERT INTO price_history (coin_id, price, currency, fetched_at) VALUES ($1, $2, $3, $4) RETURNING id",
		coinID, price, currency, fetchedAt,
	).Scan(&id)
	if err != nil {
		return PricePoint{}, err
	}
	return PricePoint{ID: id, CoinID: coinID, Price: price, Currency: currency, FetchedAt: fetchedAt}, nil
}

func listPrices(db dbExecer, coinID int, from, to string, limit, offset int) ([]PricePoint, error) {
	clauses := []string{"coin_id = $1"}
	args := []any{coinID}
	if from != "" {
		clauses = append(clauses, fmt.Sprintf("fetched_at >= $%d", len(args)+1))
		args = append(args, from)
	}
	if to != "" {
		clauses = append(clauses, fmt.Sprintf("fetched_at <= $%d", len(args)+1))
		args = append(args, to)
	}
	limitPh := fmt.Sprintf("$%d", len(args)+1)
	offsetPh := fmt.Sprintf("$%d", len(args)+2)
	args = append(args, limit, offset)

	query := "SELECT id, coin_id, price, currency, fetched_at FROM price_history WHERE " +
		strings.Join(clauses, " AND ") +
		" ORDER BY fetched_at DESC, id DESC LIMIT " + limitPh + " OFFSET " + offsetPh

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []PricePoint{}
	for rows.Next() {
		var p PricePoint
		if err := rows.Scan(&p.ID, &p.CoinID, &p.Price, &p.Currency, &p.FetchedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func countPrices(db dbExecer, coinID int, from, to string) (int, error) {
	clauses := []string{"coin_id = $1"}
	args := []any{coinID}
	if from != "" {
		clauses = append(clauses, fmt.Sprintf("fetched_at >= $%d", len(args)+1))
		args = append(args, from)
	}
	if to != "" {
		clauses = append(clauses, fmt.Sprintf("fetched_at <= $%d", len(args)+1))
		args = append(args, to)
	}
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM price_history WHERE "+strings.Join(clauses, " AND "), args...).Scan(&n)
	return n, err
}

func getLatestPrice(db dbExecer, coinID int) (PricePoint, error) {
	var p PricePoint
	err := db.QueryRow(
		"SELECT id, coin_id, price, currency, fetched_at FROM price_history WHERE coin_id = $1 ORDER BY fetched_at DESC, id DESC LIMIT 1",
		coinID,
	).Scan(&p.ID, &p.CoinID, &p.Price, &p.Currency, &p.FetchedAt)
	if err == sql.ErrNoRows {
		return PricePoint{}, ErrNotFound
	}
	return p, err
}

func listLatestPrices(db dbExecer, limit, offset int) ([]LatestPrice, error) {
	// Каждый подзапрос — index-scan по (coin_id, fetched_at DESC, id DESC): оптимально.
	rows, err := db.Query(
		`SELECT c.id AS coin_id, c.symbol, c.name, c.price_currency,
		        p.price, p.currency, p.fetched_at
		 FROM coins c
		 LEFT JOIN price_history p ON p.id = (
		   SELECT id FROM price_history
		   WHERE coin_id = c.id
		   ORDER BY fetched_at DESC, id DESC
		   LIMIT 1
		 )
		 ORDER BY c.id ASC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	latest := []LatestPrice{}
	for rows.Next() {
		var l LatestPrice
		var priceCurrency string
		var currencyCol sql.NullString
		var price sql.NullFloat64
		var fetched sql.NullString
		if err := rows.Scan(&l.CoinID, &l.Symbol, &l.Name, &priceCurrency, &price, &currencyCol, &fetched); err != nil {
			return nil, err
		}
		l.Currency = priceCurrency
		if currencyCol.Valid {
			l.Currency = currencyCol.String
		}
		if price.Valid {
			l.Price = &price.Float64
		}
		if fetched.Valid {
			l.FetchedAt = &fetched.String
		}
		latest = append(latest, l)
	}
	return latest, rows.Err()
}
