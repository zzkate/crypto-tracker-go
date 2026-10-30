package main

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeCmc struct {
	mu    sync.Mutex
	calls int
	fn    func(call int, symbols []string) (map[string]float64, error)
}

func (f *fakeCmc) fetchPrices(_ context.Context, symbols []string, _ string) (map[string]float64, error) {
	f.mu.Lock()
	f.calls++
	n := f.calls
	f.mu.Unlock()
	return f.fn(n, symbols)
}

func (f *fakeCmc) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newRetrySvc(client cmcAPI, maxRetries int) *syncService {
	return newSyncService(nil, client, "USD", 100, maxRetries, 1)
}

func TestFetchWithRetrySucceedsAfterRetries(t *testing.T) {
	client := &fakeCmc{fn: func(call int, _ []string) (map[string]float64, error) {
		if call < 3 {
			return nil, &cmcError{Status: 500, Message: "boom"}
		}
		return map[string]float64{"BTC": 42}, nil
	}}
	svc := newRetrySvc(client, 3)
	prices, err := svc.fetchWithRetry(context.Background(), []string{"BTC"})
	if err != nil || prices["BTC"] != 42 {
		t.Fatalf("prices=%v err=%v", prices, err)
	}
	if client.Calls() != 3 {
		t.Fatalf("calls = %d, want 3", client.Calls())
	}
}

func TestFetchWithRetryGivesUp(t *testing.T) {
	client := &fakeCmc{fn: func(_ int, _ []string) (map[string]float64, error) {
		return nil, &cmcError{Status: 503, Message: "down"}
	}}
	svc := newRetrySvc(client, 2)
	if _, err := svc.fetchWithRetry(context.Background(), []string{"BTC"}); err == nil {
		t.Fatal("want error after retries")
	}
	if client.Calls() != 3 {
		t.Fatalf("calls = %d, want 3 (1 initial + 2 retries)", client.Calls())
	}
}

func TestFetchWithRetryNoRetryOn4xx(t *testing.T) {
	client := &fakeCmc{fn: func(_ int, _ []string) (map[string]float64, error) {
		return nil, &cmcError{Status: 401, Message: "unauthorized"}
	}}
	svc := newRetrySvc(client, 3)
	if _, err := svc.fetchWithRetry(context.Background(), []string{"BTC"}); err == nil {
		t.Fatal("want error")
	}
	if client.Calls() != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 4xx)", client.Calls())
	}
}

func TestFetchWithRetrySuccessFirstTry(t *testing.T) {
	client := &fakeCmc{fn: func(_ int, _ []string) (map[string]float64, error) {
		return map[string]float64{"BTC": 1}, nil
	}}
	svc := newRetrySvc(client, 2)
	if _, err := svc.fetchWithRetry(context.Background(), []string{"BTC"}); err != nil {
		t.Fatal(err)
	}
	if client.Calls() != 1 {
		t.Fatalf("calls = %d, want 1", client.Calls())
	}
}

func TestFetchWithRetryNetworkErrorRetried(t *testing.T) {
	client := &fakeCmc{fn: func(call int, _ []string) (map[string]float64, error) {
		if call == 1 {
			return nil, errors.New("connection refused")
		}
		return map[string]float64{"BTC": 9}, nil
	}}
	svc := newRetrySvc(client, 2)
	prices, err := svc.fetchWithRetry(context.Background(), []string{"BTC"})
	if err != nil || prices["BTC"] != 9 {
		t.Fatalf("prices=%v err=%v", prices, err)
	}
	if client.Calls() != 2 {
		t.Fatalf("calls = %d, want 2", client.Calls())
	}
}
