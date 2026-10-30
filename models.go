package main

// Coin — модель таблицы coins.
type Coin struct {
	ID            int64   `gorm:"column:id;primaryKey" json:"id"`
	Symbol        string  `gorm:"column:symbol;uniqueIndex;not null" json:"symbol"`
	Name          string  `gorm:"column:name;not null" json:"name"`
	PriceCurrency string  `gorm:"column:price_currency;not null" json:"priceCurrency"`
	LastUpdatedAt *string `gorm:"column:last_updated_at" json:"lastUpdatedAt"`
	CreatedAt     string  `gorm:"column:created_at;not null" json:"createdAt"`
}

func (Coin) TableName() string { return "coins" }

// PricePoint — модель таблицы price_history.
type PricePoint struct {
	ID        int64   `gorm:"column:id;primaryKey" json:"id"`
	CoinID    int64   `gorm:"column:coin_id;not null" json:"coinId"`
	Price     float64 `gorm:"column:price;not null" json:"price"`
	Currency  string  `gorm:"column:currency;not null" json:"currency"`
	FetchedAt string  `gorm:"column:fetched_at;not null" json:"fetchedAt"`
}

func (PricePoint) TableName() string { return "price_history" }

// LatestPrice — результат запроса «последние цены всех монет» (не таблица).
type LatestPrice struct {
	CoinID    int64    `json:"coinId"`
	Symbol    string   `json:"symbol"`
	Name      string   `json:"name"`
	Currency  string   `json:"currency"`
	Price     *float64 `json:"price"`
	FetchedAt *string  `json:"fetchedAt"`
}
