package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestGracefulShutdownWaitsForInflight(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: h}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	done := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			done <- -1
			return
		}
		done <- resp.StatusCode
	}()

	<-started

	shutdownCh := make(chan error, 1)
	go func() { shutdownCh <- shutdownServer(srv, 5*time.Second) }()

	// Shutdown не должен завершаться, пока висит in-flight запрос.
	select {
	case err := <-shutdownCh:
		t.Fatalf("shutdown вернулся до завершения запроса: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if code := <-done; code != http.StatusOK {
		t.Fatalf("код запроса = %d, want 200", code)
	}
	if err := <-shutdownCh; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestGracefulShutdownTimeout(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // никогда не отпускаем в этом тесте
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: h}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		http.Get("http://" + ln.Addr().String() + "/")
	}()
	<-started

	begin := time.Now()
	err = shutdownServer(srv, 100*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown err = %v, want DeadlineExceeded", err)
	}
	if elapsed := time.Since(begin); elapsed < 100*time.Millisecond {
		t.Fatalf("shutdown вернулся слишком рано: %v", elapsed)
	}

	close(release)
	<-clientDone
}
