package main

import (
	"context"
	"net/http"
	"time"
)

// shutdownServer мягко гасит сервер: дожидается завершения текущих запросов,
// но не дольше timeout (иначе вернёт ошибку контекста).
func shutdownServer(server *http.Server, timeout time.Duration) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
