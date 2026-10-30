package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	loadDotEnv(".env")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := openDBWithRetry(cfg.DatabaseURL, 10, time.Second)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	var client cmcAPI
	if cfg.CmcAPIKey != "" {
		client = newCmcClient(cfg.CmcAPIKey, cfg.CmcBaseURL, cfg.CmcTimeoutMS)
	} else {
		client = disabledClient{}
		log.Println("CMC_API_KEY is not set: background sync disabled")
	}
	syncSvc := newSyncService(db, client, cfg.Currency, cfg.CmcBatchSize, cfg.CmcMaxRetries, cfg.CmcRetryBackoff)

	handler := newApp(db, cfg, syncSvc).routes()

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.CmcAPIKey != "" {
		startBackgroundSync(ctx, syncSvc, cfg.SyncIntervalMS, log.Default())
	}

	go func() {
		log.Printf("listening on :%d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	if err := shutdownServer(server, 10*time.Second); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}

type disabledClient struct{}

func (disabledClient) fetchPrices(ctx context.Context, symbols []string, currency string) (map[string]float64, error) {
	return nil, &cmcError{Status: 400, Message: "CMC_API_KEY is not configured"}
}
