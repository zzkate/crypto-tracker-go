package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type mockCmc struct {
	mu    sync.Mutex
	fn    func(ctx context.Context, symbols []string, currency string) (map[string]float64, error)
	calls []string
}

func (m *mockCmc) fetchPrices(ctx context.Context, symbols []string, currency string) (map[string]float64, error) {
	if m.mu.TryLock() {
		m.calls = append(m.calls, strings.Join(symbols, ","))
		m.mu.Unlock()
	}
	return m.fn(ctx, symbols, currency)
}

func newTestServer(t *testing.T, client cmcAPI) (http.Handler, *app, *sql.DB, *syncService) {
	t.Helper()
	db, err := openDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		client = disabledClient{}
	}
	cfg := Config{
		AppAPIKey: "test-key",
		Currency:  "USD",
	}
	syncSvc := newSyncService(db, client, cfg.Currency, 100, 0, 200)
	app := newApp(db, cfg, syncSvc)
	return app.routes(), app, db, syncSvc
}

func doReq(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}
	return v
}

// --- auth ---

func TestHealthAndAuth(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	if rec := doReq(t, h, http.MethodGet, "/health", ""); rec.Code != 200 {
		t.Fatalf("health = %d", rec.Code)
	}

	// без токена
	req := httptest.NewRequest(http.MethodGet, "/api/coins", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("no token = %d, want 401", rec.Code)
	}

	if rec := doReq(t, h, http.MethodGet, "/api/coins", ""); rec.Code != 200 {
		t.Fatalf("coins = %d", rec.Code)
	}
}

// --- coins CRUD ---

func TestCoinsCRUD(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"btc","name":"Bitcoin"}`)
	if rec.Code != 201 {
		t.Fatalf("create = %d", rec.Code)
	}
	coin := decode[Coin](t, rec)
	if coin.Symbol != "BTC" || coin.LastUpdatedAt != nil || coin.ID == 0 {
		t.Fatalf("unexpected coin: %+v", coin)
	}

	// дубликат
	if rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"dup"}`); rec.Code != 409 {
		t.Fatalf("duplicate = %d, want 409", rec.Code)
	}

	// список
	rec = doReq(t, h, http.MethodGet, "/api/coins", "")
	if rec.Code != 200 || rec.Header().Get("X-Total-Count") != "1" {
		t.Fatalf("list = %d / %s", rec.Code, rec.Header().Get("X-Total-Count"))
	}

	// PATCH идемпотентный
	rec = doReq(t, h, http.MethodPatch, "/api/coins/1", `{"symbol":"BTC","name":"Bitcoin"}`)
	if rec.Code != 200 || rec.Header().Get("X-Changed") != "false" {
		t.Fatalf("patch noop = %d / %s", rec.Code, rec.Header().Get("X-Changed"))
	}
	rec = doReq(t, h, http.MethodPatch, "/api/coins/1", `{"name":"Bitcoin updated"}`)
	if rec.Code != 200 || rec.Header().Get("X-Changed") != "true" {
		t.Fatalf("patch change = %d / %s", rec.Code, rec.Header().Get("X-Changed"))
	}

	// get
	rec = doReq(t, h, http.MethodGet, "/api/coins/1", "")
	if rec.Code != 200 || decode[Coin](t, rec).Name != "Bitcoin updated" {
		t.Fatalf("get = %d", rec.Code)
	}

	// 404
	if rec := doReq(t, h, http.MethodGet, "/api/coins/999", ""); rec.Code != 404 {
		t.Fatalf("get missing = %d", rec.Code)
	}

	// delete
	if rec := doReq(t, h, http.MethodDelete, "/api/coins/1", ""); rec.Code != 204 {
		t.Fatalf("delete = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/api/coins/1", ""); rec.Code != 404 {
		t.Fatalf("get after delete = %d", rec.Code)
	}
}

// --- validation / поломка ---

func TestValidation(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	cases := []struct {
		method, path, body string
		want               int
	}{
		{http.MethodPost, "/api/coins", `{"symbol":"b","name":"x"}`, 400},
		{http.MethodPost, "/api/coins", `{"symbol":"BTC"}`, 400},
		{http.MethodPost, "/api/coins", `{"symbol":"BTC","name":""}`, 400},
		{http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"x"}`, 201},
		{http.MethodPost, "/api/coins", `42`, 400},
		{http.MethodPost, "/api/coins", `"str"`, 400},
		{http.MethodPost, "/api/coins", `null`, 400},
		{http.MethodPost, "/api/coins", `{"symbol":"BTC",`, 400}, // битый JSON
		{http.MethodGet, "/api/coins/abc", "", 400},
		{http.MethodGet, "/api/coins", "", 200},
	}
	for _, c := range cases {
		if rec := doReq(t, h, c.method, c.path, c.body); rec.Code != c.want {
			t.Errorf("%s %s = %d, want %d", c.method, c.path, rec.Code, c.want)
		}
	}

	// слишком большое тело -> 413
	big := `{"symbol":"BTC","name":"` + strings.Repeat("a", 20000) + `"}`
	if rec := doReq(t, h, http.MethodPost, "/api/coins", big); rec.Code != 413 {
		t.Fatalf("oversized = %d, want 413", rec.Code)
	}
}

func TestSQLInjection(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	payload := "x'; DROP TABLE coins;--"
	rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"`+payload+`"}`)
	if rec.Code != 201 || decode[Coin](t, rec).Name != payload {
		t.Fatalf("injection name = %d", rec.Code)
	}

	// таблица цела
	if rec := doReq(t, h, http.MethodGet, "/api/coins", ""); rec.Code != 200 {
		t.Fatalf("coins after injection = %d", rec.Code)
	}

	// инъекции в symbol/id/limit
	if rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"'; DROP TABLE coins;--","name":"x"}`); rec.Code != 400 {
		t.Fatalf("symbol injection = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/api/coins/1%20OR%201=1", ""); rec.Code != 400 {
		t.Fatalf("id injection = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/api/coins/1/prices?limit=1%20OR%201=1", ""); rec.Code != 400 {
		t.Fatalf("limit injection = %d", rec.Code)
	}
}

// --- prices ---

func TestPrices(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"Bitcoin"}`)
	doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"ETH","name":"Ethereum"}`)

	// до синхронизации — null
	rec := doReq(t, h, http.MethodGet, "/api/prices/latest", "")
	if rec.Code != 200 {
		t.Fatalf("latest = %d", rec.Code)
	}
	latest := decode[[]LatestPrice](t, rec)
	if len(latest) != 2 || latest[0].Price != nil {
		t.Fatalf("unexpected latest: %+v", latest)
	}

	// добавляем историю напрямую
	if _, err := insertPrice(db, 1, 100, "USD", "2026-01-01T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := insertPrice(db, 1, 150, "USD", "2026-01-03T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := insertPrice(db, 1, 120, "USD", "2026-01-02T10:00:00Z"); err != nil {
		t.Fatal(err)
	}

	rec = doReq(t, h, http.MethodGet, "/api/prices/latest", "")
	latest = decode[[]LatestPrice](t, rec)
	if latest[0].Price == nil || *latest[0].Price != 150 {
		t.Fatalf("latest BTC = %v, want 150", latest[0].Price)
	}

	// история с фильтрами
	rec = doReq(t, h, http.MethodGet, "/api/coins/1/prices?from=2026-01-02T00:00:00Z", "")
	history := decode[[]PricePoint](t, rec)
	if len(history) != 2 || history[0].Price != 150 || history[1].Price != 120 {
		t.Fatalf("history from = %+v", history)
	}

	// latest по одной монете
	rec = doReq(t, h, http.MethodGet, "/api/coins/1/prices/latest", "")
	if rec.Code != 200 || decode[PricePoint](t, rec).Price != 150 {
		t.Fatalf("coin latest = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/api/coins/2/prices/latest", ""); rec.Code != 404 {
		t.Fatalf("coin latest (no price) = %d", rec.Code)
	}
}

// --- пагинация ---

func TestPagination(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	for _, s := range []string{"S0", "S1", "S2", "S3", "S4"} {
		doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"`+s+`","name":"`+s+`"}`)
	}

	rec := doReq(t, h, http.MethodGet, "/api/coins?limit=2&offset=1", "")
	if rec.Code != 200 || rec.Header().Get("X-Total-Count") != "5" {
		t.Fatalf("page = %d / %s", rec.Code, rec.Header().Get("X-Total-Count"))
	}
	coins := decode[[]Coin](t, rec)
	if len(coins) != 2 || coins[0].Symbol != "S1" || coins[1].Symbol != "S2" {
		t.Fatalf("page coins = %+v", coins)
	}

	if rec := doReq(t, h, http.MethodGet, "/api/coins?offset=-1", ""); rec.Code != 400 {
		t.Fatalf("negative offset = %d", rec.Code)
	}
}

// --- sync ---

func TestSyncOnce(t *testing.T) {
	client := &mockCmc{fn: func(_ context.Context, symbols []string, currency string) (map[string]float64, error) {
		m := map[string]float64{}
		for _, s := range symbols {
			m[s] = 42
		}
		return m, nil
	}}
	h, _, db, svc := newTestServer(t, client)
	defer db.Close()

	doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"Bitcoin"}`)
	doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"ETH","name":"Ethereum"}`)

	result, err := svc.syncOnce(context.Background())
	if err != nil || result.Updated != 2 || result.Skipped != 0 {
		t.Fatalf("sync = %+v, %v", result, err)
	}

	rec := doReq(t, h, http.MethodGet, "/api/prices/latest", "")
	latest := decode[[]LatestPrice](t, rec)
	if len(latest) != 2 || latest[0].Price == nil || *latest[0].Price != 42 {
		t.Fatalf("latest after sync = %+v", latest)
	}
}

func TestSyncBatching(t *testing.T) {
	client := &mockCmc{fn: func(_ context.Context, symbols []string, currency string) (map[string]float64, error) {
		m := map[string]float64{}
		for _, s := range symbols {
			m[s] = 1
		}
		return m, nil
	}}
	db, _ := openDB(":memory:")
	defer db.Close()
	svc := newSyncService(db, client, "USD", 2, 0, 200)

	for i := 0; i < 5; i++ {
		if _, err := createCoin(db, "S"+string(rune('0'+i)), "Coin", "USD", "2026-01-01T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := svc.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 5 монет, batch 2 → 3 вызова
	if len(client.calls) != 3 {
		t.Fatalf("calls = %d, want 3 (%v)", len(client.calls), client.calls)
	}
}

func TestSyncUpstreamError(t *testing.T) {
	client := &mockCmc{fn: func(_ context.Context, symbols []string, currency string) (map[string]float64, error) {
		return nil, &cmcError{Status: 500, Message: "upstream down"}
	}}
	h, _, db, _ := newTestServer(t, client)
	defer db.Close()

	doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"Bitcoin"}`)

	if rec := doReq(t, h, http.MethodPost, "/api/sync", ""); rec.Code != 502 {
		t.Fatalf("sync error = %d, want 502", rec.Code)
	}
}

func TestIsRetryableError(t *testing.T) {
	if !isRetryableError(errors.New("network")) {
		t.Fatal("network should be retryable")
	}
	if !isRetryableError(&cmcError{Status: 500}) || !isRetryableError(&cmcError{Status: 429}) {
		t.Fatal("5xx/429 should be retryable")
	}
	if isRetryableError(&cmcError{Status: 401}) || isRetryableError(&cmcError{Status: 404}) {
		t.Fatal("4xx should NOT be retryable")
	}
}

func TestSyncSkipOnConcurrent(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	client := &mockCmc{fn: func(_ context.Context, symbols []string, currency string) (map[string]float64, error) {
		close(started)
		<-release
		return map[string]float64{"BTC": 1}, nil
	}}
	db, _ := openDB(":memory:")
	defer db.Close()
	svc := newSyncService(db, client, "USD", 100, 0, 200)
	if _, err := createCoin(db, "BTC", "Bitcoin", "USD", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		_, _ = svc.syncOnce(context.Background())
		close(done)
	}()
	<-started

	if _, err := svc.syncOnce(context.Background()); !errors.Is(err, ErrSyncInProgress) {
		t.Fatalf("second sync err = %v, want ErrSyncInProgress", err)
	}
	close(release)
	<-done
}

// --- metrics / docs ---

func TestMetricsAndDocs(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer db.Close()

	if rec := doReq(t, h, http.MethodGet, "/metrics", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "http_requests_total") {
		t.Fatalf("metrics = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/openapi.json", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"openapi"`) || !strings.Contains(rec.Body.String(), "3.0.3") {
		t.Fatalf("openapi = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/docs/", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "swagger") {
		t.Fatalf("docs = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodGet, "/nope", ""); rec.Code != 404 {
		t.Fatalf("unknown = %d", rec.Code)
	}
}
