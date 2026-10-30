package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

type cmcAPI interface {
	fetchPrices(ctx context.Context, symbols []string, currency string) (map[string]float64, error)
}

// isRetryableError: ретраим сеть/таймаут и 5xx/429; 4xx (401/403/404…) — нет.
func isRetryableError(err error) bool {
	var ce *cmcError
	if errors.As(err, &ce) {
		return ce.Status == 429 || ce.Status >= 500
	}
	return true
}

type syncService struct {
	db           *gorm.DB
	client       cmcAPI
	currency     string
	batchSize    int
	maxRetries   int
	retryBackoff time.Duration

	mu       sync.Mutex
	inFlight bool
}

type syncResult struct {
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

func newSyncService(db *gorm.DB, client cmcAPI, currency string, batchSize, maxRetries, retryBackoffMS int) *syncService {
	return &syncService{
		db:           db,
		client:       client,
		currency:     currency,
		batchSize:    batchSize,
		maxRetries:   maxRetries,
		retryBackoff: time.Duration(retryBackoffMS) * time.Millisecond,
	}
}

func (s *syncService) syncOnce(ctx context.Context) (syncResult, error) {
	s.mu.Lock()
	if s.inFlight {
		s.mu.Unlock()
		return syncResult{}, ErrSyncInProgress
	}
	s.inFlight = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.inFlight = false
		s.mu.Unlock()
	}()

	coins, err := allCoins(s.db)
	if err != nil {
		return syncResult{}, err
	}
	if len(coins) == 0 {
		return syncResult{}, nil
	}

	prices := map[string]float64{}
	for i := 0; i < len(coins); i += s.batchSize {
		end := i + s.batchSize
		if end > len(coins) {
			end = len(coins)
		}
		symbols := make([]string, 0, end-i)
		for _, c := range coins[i:end] {
			symbols = append(symbols, c.Symbol)
		}
		chunk, err := s.fetchWithRetry(ctx, symbols)
		if err != nil {
			return syncResult{}, err
		}
		for k, v := range chunk {
			prices[k] = v
		}
	}

	fetchedAt := canonTime(time.Now())

	// Пишем в одной транзакции: атомарно и быстрее, чем N автокоммитов.
	result := syncResult{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range coins {
			p, ok := prices[c.Symbol]
			if !ok {
				result.Skipped++
				continue
			}
			if _, err := insertPrice(tx, c.ID, p, s.currency, fetchedAt); err != nil {
				return err
			}
			if err := touchCoin(tx, c.ID, fetchedAt); err != nil {
				return err
			}
			result.Updated++
		}
		return nil
	})
	if err != nil {
		return syncResult{}, err
	}
	return result, nil
}

func (s *syncService) fetchWithRetry(ctx context.Context, symbols []string) (map[string]float64, error) {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		prices, err := s.client.fetchPrices(ctx, symbols, s.currency)
		if err == nil {
			return prices, nil
		}
		lastErr = err
		if attempt < s.maxRetries && isRetryableError(err) {
			select {
			case <-time.After(s.retryBackoff * time.Duration(1<<attempt)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		break
	}
	return nil, lastErr
}

// startBackgroundSync запускает синхронизацию сразу и далее по интервалу до отмены ctx.
func startBackgroundSync(ctx context.Context, svc *syncService, intervalMS int, logger *log.Logger) {
	run := func() {
		if _, err := svc.syncOnce(context.Background()); err != nil && !errors.Is(err, ErrSyncInProgress) {
			logger.Printf("[sync] failed: %v", err)
		}
	}

	go func() {
		run() // первый прогон сразу
		ticker := time.NewTicker(time.Duration(intervalMS) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
