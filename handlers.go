package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

//go:embed openapi.json
var openapiJSON []byte

//go:embed docs/index.html
var docsHTML []byte

type app struct {
	db      *sql.DB
	config  Config
	syncSvc *syncService
}

func newApp(db *sql.DB, config Config, syncSvc *syncService) *app {
	return &app{db: db, config: config, syncSvc: syncSvc}
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /metrics", metricsHandler(a.db))
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openapiJSON)
	})
	mux.HandleFunc("GET /docs", serveDocs)
	mux.HandleFunc("GET /docs/", serveDocs)

	auth := a.config.AppAPIKey
	mux.HandleFunc("GET /api/coins", requireAuth(auth, a.listCoins))
	mux.HandleFunc("POST /api/coins", requireAuth(auth, a.createCoin))
	mux.HandleFunc("GET /api/coins/{id}", requireAuth(auth, a.getCoin))
	mux.HandleFunc("PATCH /api/coins/{id}", requireAuth(auth, a.updateCoin))
	mux.HandleFunc("DELETE /api/coins/{id}", requireAuth(auth, a.deleteCoin))
	mux.HandleFunc("GET /api/coins/{id}/prices", requireAuth(auth, a.listPrices))
	mux.HandleFunc("GET /api/coins/{id}/prices/latest", requireAuth(auth, a.latestPrice))
	mux.HandleFunc("GET /api/prices/latest", requireAuth(auth, a.latestPrices))
	mux.HandleFunc("POST /api/sync", requireAuth(auth, a.manualSync))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
	})

	return metricsMiddleware(mux)
}

func serveDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(docsHTML)
}

func (a *app) listCoins(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r.URL.Query())
	if err != nil {
		a.handleError(w, err)
		return
	}
	total, err := countCoins(a.db)
	if err != nil {
		a.handleError(w, err)
		return
	}
	coins, err := listCoins(a.db, limit, offset)
	if err != nil {
		a.handleError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, coins)
}

func (a *app) createCoin(w http.ResponseWriter, r *http.Request) {
	symbol, name, err := a.readCoinBody(w, r, false)
	if err != nil {
		a.handleError(w, err)
		return
	}
	coin, err := createCoin(a.db, *symbol, *name, a.config.Currency, canonTime(time.Now()))
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "coin with this symbol already exists")
			return
		}
		a.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, coin)
}

func (a *app) getCoin(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		a.handleError(w, err)
		return
	}
	coin, err := getCoin(a.db, id)
	if err != nil {
		a.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, coin)
}

func (a *app) updateCoin(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		a.handleError(w, err)
		return
	}
	symbol, name, err := a.readCoinBody(w, r, true)
	if err != nil {
		a.handleError(w, err)
		return
	}

	current, err := getCoin(a.db, id)
	if err != nil {
		a.handleError(w, err)
		return
	}

	nextSymbol, nextName := current.Symbol, current.Name
	if symbol != nil {
		nextSymbol = *symbol
	}
	if name != nil {
		nextName = *name
	}

	// PATCH идемпотентен: если данные не изменились, UPDATE не выполняем.
	if nextSymbol == current.Symbol && nextName == current.Name {
		w.Header().Set("X-Changed", "false")
		writeJSON(w, http.StatusOK, current)
		return
	}

	coin, err := updateCoin(a.db, id, symbol, name)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "coin with this symbol already exists")
			return
		}
		a.handleError(w, err)
		return
	}
	w.Header().Set("X-Changed", "true")
	writeJSON(w, http.StatusOK, coin)
}

func (a *app) deleteCoin(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		a.handleError(w, err)
		return
	}
	deleted, err := deleteCoin(a.db, id)
	if err != nil {
		a.handleError(w, err)
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "not_found", "coin not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) listPrices(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		a.handleError(w, err)
		return
	}
	if _, err := getCoin(a.db, id); err != nil {
		a.handleError(w, err)
		return
	}

	q := r.URL.Query()
	from, err := parseDate(q, "from")
	if err != nil {
		a.handleError(w, err)
		return
	}
	to, err := parseDate(q, "to")
	if err != nil {
		a.handleError(w, err)
		return
	}
	if from != "" && to != "" && from > to {
		a.handleError(w, badRequest("from must be earlier than or equal to to"))
		return
	}
	limit, offset, err := parsePagination(q)
	if err != nil {
		a.handleError(w, err)
		return
	}

	total, err := countPrices(a.db, id, from, to)
	if err != nil {
		a.handleError(w, err)
		return
	}
	points, err := listPrices(a.db, id, from, to, limit, offset)
	if err != nil {
		a.handleError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, points)
}

func (a *app) latestPrice(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		a.handleError(w, err)
		return
	}
	if _, err := getCoin(a.db, id); err != nil {
		a.handleError(w, err)
		return
	}
	p, err := getLatestPrice(a.db, id)
	if err != nil {
		a.handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *app) latestPrices(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r.URL.Query())
	if err != nil {
		a.handleError(w, err)
		return
	}
	total, err := countCoins(a.db)
	if err != nil {
		a.handleError(w, err)
		return
	}
	latest, err := listLatestPrices(a.db, limit, offset)
	if err != nil {
		a.handleError(w, err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, latest)
}

func (a *app) manualSync(w http.ResponseWriter, r *http.Request) {
	result, err := a.syncSvc.syncOnce(r.Context())
	if err != nil {
		if errors.Is(err, ErrSyncInProgress) {
			writeError(w, http.StatusConflict, "sync_in_progress", "sync already in progress")
			return
		}
		recordSyncFailure()
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	recordSyncSuccess()
	writeJSON(w, http.StatusOK, result)
}

func (a *app) readCoinBody(w http.ResponseWriter, r *http.Request, partial bool) (symbol, name *string, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, nil, &httpError{Status: http.StatusRequestEntityTooLarge, Code: "payload_too_large", Message: "request body is too large"}
		}
		return nil, nil, err
	}
	return parseCoinInput(body, partial)
}

func (a *app) handleError(w http.ResponseWriter, err error) {
	var he *httpError
	if errors.As(err, &he) {
		writeError(w, he.Status, he.Code, he.Message)
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	log.Printf("[error] %v", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
