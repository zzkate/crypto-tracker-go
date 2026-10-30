package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaginationBounds(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer closeDB(db)

	for _, q := range []string{"?limit=0", "?limit=-1", "?limit=1001", "?limit=abc", "?offset=-1", "?offset=abc"} {
		rec := doReq(t, h, http.MethodGet, "/api/coins"+q, "")
		if rec.Code != 400 {
			t.Errorf("GET /api/coins%s = %d, want 400", q, rec.Code)
		}
	}
}

func TestUpdateCoinDuplicateSymbol(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer closeDB(db)

	if rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"BTC","name":"Bitcoin"}`); rec.Code != 201 {
		t.Fatalf("create BTC = %d", rec.Code)
	}
	if rec := doReq(t, h, http.MethodPost, "/api/coins", `{"symbol":"ETH","name":"Ethereum"}`); rec.Code != 201 {
		t.Fatalf("create ETH = %d", rec.Code)
	}

	// PATCH ETH -> BTC (уже занят) → 409.
	if rec := doReq(t, h, http.MethodPatch, "/api/coins/2", `{"symbol":"BTC"}`); rec.Code != 409 {
		t.Fatalf("patch duplicate = %d, want 409", rec.Code)
	}
}

func TestAuthWrongToken(t *testing.T) {
	h, _, db, _ := newTestServer(t, nil)
	defer closeDB(db)

	req := httptest.NewRequest(http.MethodGet, "/api/coins", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("wrong token = %d, want 401", rec.Code)
	}

	// не-Bearer схема тоже отклоняется
	req = httptest.NewRequest(http.MethodGet, "/api/coins", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("non-bearer = %d, want 401", rec.Code)
	}
}
