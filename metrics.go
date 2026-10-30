package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

var (
	httpRequests  atomic.Int64
	httpErrors    atomic.Int64
	httpDurMsSum  atomic.Int64
	syncTotal     atomic.Int64
	syncFailed    atomic.Int64
	syncLastEpoch atomic.Int64
)

func recordSyncSuccess() {
	syncTotal.Add(1)
	syncLastEpoch.Store(time.Now().Unix())
}

func recordSyncFailure() {
	syncTotal.Add(1)
	syncFailed.Add(1)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		httpRequests.Add(1)
		if rec.status >= 400 {
			httpErrors.Add(1)
		}
		httpDurMsSum.Add(time.Since(start).Milliseconds())
	})
}

func metricsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		coins, _ := countCoins(db)
		var points int64
		_ = db.Model(&PricePoint{}).Count(&points).Error

		reqs := httpRequests.Load()
		avg := int64(0)
		if reqs > 0 {
			avg = httpDurMsSum.Load() / reqs
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintf(w, `# HELP http_requests_total Общее число HTTP-запросов.
# TYPE http_requests_total counter
http_requests_total %d
# HELP http_errors_total Число ответов 4xx/5xx.
# TYPE http_errors_total counter
http_errors_total %d
# HELP http_request_duration_ms_avg Средняя длительность запроса, мс.
# TYPE http_request_duration_ms_avg gauge
http_request_duration_ms_avg %d
# HELP sync_total Всего синхронизаций.
# TYPE sync_total counter
sync_total %d
# HELP sync_failed_total Неудачных синхронизаций.
# TYPE sync_failed_total counter
sync_failed_total %d
# HELP sync_last_success_epoch Время последней успешной синхронизации (unix).
# TYPE sync_last_success_epoch gauge
sync_last_success_epoch %d
# HELP coins_total Отслеживаемых монет.
# TYPE coins_total gauge
coins_total %d
# HELP price_points_total Точек в истории цен.
# TYPE price_points_total gauge
price_points_total %d
`, reqs, httpErrors.Load(), avg, syncTotal.Load(), syncFailed.Load(), syncLastEpoch.Load(), coins, points)
	}
}
