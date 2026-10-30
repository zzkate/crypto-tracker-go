package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"gorm.io/gorm"
)

const benchFetchedAt = "2026-01-01T10:00:00Z"

// benchCmc — лёгкий cmcAPI для бенчмарков (без записи вызовов).
type benchCmc struct {
	fn func(ctx context.Context, symbols []string, currency string) (map[string]float64, error)
}

func (b *benchCmc) fetchPrices(ctx context.Context, symbols []string, currency string) (map[string]float64, error) {
	return b.fn(ctx, symbols, currency)
}

// --- репозитории ---

func BenchmarkCreateCoin(b *testing.B) {
	db := openTestDB(b)
	defer closeDB(db)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := createCoin(db, "B"+strconv.Itoa(i), "Coin", "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetCoin(b *testing.B) {
	db := openTestDB(b)
	defer closeDB(db)
	c, err := createCoin(db, "BTC", "Bitcoin", "USD", benchFetchedAt)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := getCoin(db, c.ID); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListCoins(b *testing.B) {
	db := openTestDB(b)
	defer closeDB(db)
	for i := 0; i < 200; i++ {
		if _, err := createCoin(db, "C"+strconv.Itoa(i), "Coin", "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := listCoins(db, 50, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertPrice(b *testing.B) {
	db := openTestDB(b)
	defer closeDB(db)
	c, err := createCoin(db, "BTC", "Bitcoin", "USD", benchFetchedAt)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := insertPrice(db, c.ID, float64(i%1000), "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListLatestPrices(b *testing.B) {
	db := openTestDB(b)
	defer closeDB(db)
	for i := 0; i < 200; i++ {
		c, err := createCoin(db, "L"+strconv.Itoa(i), "Coin", "USD", benchFetchedAt)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := insertPrice(db, c.ID, 42, "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := listLatestPrices(db, 50, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListPrices(b *testing.B) {
	db := openTestDB(b)
	defer closeDB(db)
	c, err := createCoin(db, "BTC", "Bitcoin", "USD", benchFetchedAt)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := insertPrice(db, c.ID, float64(i), "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := listPrices(db, c.ID, "", "", 50, 0); err != nil {
			b.Fatal(err)
		}
	}
}

// --- HTTP-обработчики (полный путь: маршрут + auth + JSON + БД) ---

func benchServer(b *testing.B, client cmcAPI) (http.Handler, *gorm.DB) {
	db := openTestDB(b)
	if client == nil {
		client = disabledClient{}
	}
	cfg := Config{AppAPIKey: "test-key", Currency: "USD"}
	svc := newSyncService(db, client, cfg.Currency, 100, 0, 200)
	return newApp(db, cfg, svc).routes(), db
}

func BenchmarkHTTPListCoins(b *testing.B) {
	h, db := benchServer(b, nil)
	defer closeDB(db)
	for i := 0; i < 100; i++ {
		if _, err := createCoin(db, "H"+strconv.Itoa(i), "Coin", "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/coins?limit=50", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("code = %d", rec.Code)
		}
	}
}

func BenchmarkHTTPLatestPrices(b *testing.B) {
	h, db := benchServer(b, nil)
	defer closeDB(db)
	for i := 0; i < 100; i++ {
		c, err := createCoin(db, "P"+strconv.Itoa(i), "Coin", "USD", benchFetchedAt)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := insertPrice(db, c.ID, 42, "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/prices/latest?limit=50", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("code = %d", rec.Code)
		}
	}
}

func BenchmarkHTTPSync(b *testing.B) {
	client := &benchCmc{fn: func(_ context.Context, symbols []string, _ string) (map[string]float64, error) {
		m := map[string]float64{}
		for _, s := range symbols {
			m[s] = 42
		}
		return m, nil
	}}
	h, db := benchServer(b, client)
	defer closeDB(db)
	for i := 0; i < 50; i++ {
		if _, err := createCoin(db, "S"+strconv.Itoa(i), "Coin", "USD", benchFetchedAt); err != nil {
			b.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sync", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			b.Fatalf("code = %d body = %s", rec.Code, rec.Body.String())
		}
	}
}
