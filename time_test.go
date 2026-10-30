package main

import (
	"net/http"
	"testing"
)

func TestNormTime(t *testing.T) {
	cases := map[string]string{
		"2026-01-01T10:00:00Z":      "2026-01-01T10:00:00.000000000Z",
		"2026-01-01T10:00:00.5Z":    "2026-01-01T10:00:00.500000000Z",
		"2026-01-01T10:00:00.123Z":  "2026-01-01T10:00:00.123000000Z",
		"2026-01-01T10:00:00+02:00": "2026-01-01T08:00:00.000000000Z",
		"not-a-time":                "not-a-time",
	}
	for in, want := range cases {
		if got := normTime(in); got != want {
			t.Errorf("normTime(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPricesFilterSubSecondBoundary — регрессия: подсекундные значения fetched_at
// не должны «выпадать» из фильтров from/to на границе секунды.
func TestPricesFilterSubSecondBoundary(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	if rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"Bitcoin"}`); rec.Code != 201 {
		t.Fatalf("create = %d", rec.Code)
	}
	if _, err := insertPrice(db, 1, 100, "USD", "2026-01-01T10:00:00.5Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := insertPrice(db, 1, 200, "USD", "2026-01-01T10:00:01Z"); err != nil {
		t.Fatal(err)
	}

	// from=10:00:00Z должен включить цену .5Z (она ПОЗЖЕ границы).
	rec := doReq(t, h, http.MethodGet, "/api/coins/1/prices?from=2026-01-01T10:00:00Z", "")
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	if pts := decode[[]PricePoint](t, rec); len(pts) != 2 {
		t.Fatalf("from boundary вернул %d элементов, want 2 (подсекундная цена потеряна): %+v", len(pts), pts)
	}

	// to=10:00:00Z не должен включить ничего (обе цены позже границы).
	rec = doReq(t, h, http.MethodGet, "/api/coins/1/prices?to=2026-01-01T10:00:00Z", "")
	if pts := decode[[]PricePoint](t, rec); len(pts) != 0 {
		t.Fatalf("to boundary вернул %d элементов, want 0: %+v", len(pts), pts)
	}
}

// TestLatestPriceMixedPrecision — ORDER BY fetched_at должен быть хронологическим,
// даже если точность значений отличается.
func TestLatestPriceMixedPrecision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	c, err := createCoin(db, "BTC", "Bitcoin", "USD", "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := insertPrice(db, c.ID, 100, "USD", "2026-01-01T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := insertPrice(db, c.ID, 200, "USD", "2026-01-01T10:00:00.5Z"); err != nil {
		t.Fatal(err)
	}

	p, err := getLatestPrice(db, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Price != 200 {
		t.Fatalf("latest = %v, want 200 (подсекундная цена позже)", p.Price)
	}
}

// TestPricesFilterTimezoneNormalized — фильтр с таймзоной приводится к UTC.
func TestPricesFilterTimezoneNormalized(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	if rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"Bitcoin"}`); rec.Code != 201 {
		t.Fatalf("create = %d", rec.Code)
	}
	if _, err := insertPrice(db, 1, 100, "USD", "2026-01-01T10:00:00Z"); err != nil {
		t.Fatal(err)
	}

	// 11:00+01:00 == 10:00Z — цена должна попасть в выборку.
	rec := doReq(t, h, http.MethodGet, "/api/coins/1/prices?from=2026-01-01T11:00:00%2B01:00", "")
	if pts := decode[[]PricePoint](t, rec); len(pts) != 1 {
		t.Fatalf("timezone filter вернул %d элементов, want 1: %+v", len(pts), pts)
	}
}
