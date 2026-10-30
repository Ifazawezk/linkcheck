package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/no-head", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	})
	mux.HandleFunc("/limited", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	return httptest.NewServer(mux)
}

func TestCheckAll(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	c := NewChecker(4, 100*time.Millisecond)
	urls := []string{srv.URL + "/ok", srv.URL + "/no-head", srv.URL + "/missing", srv.URL + "/slow", srv.URL + "/limited"}
	got := c.CheckAll(context.Background(), urls)

	wantBroken := []bool{false, false, true, true, false}
	for i, r := range got {
		if r.URL != urls[i] {
			t.Errorf("result %d is for %s, want %s", i, r.URL, urls[i])
		}
		if r.Broken() != wantBroken[i] {
			t.Errorf("%s: broken=%v (status %d, err %q), want %v", urls[i], r.Broken(), r.Status, r.Err, wantBroken[i])
		}
	}
	if got[2].Status != http.StatusNotFound {
		t.Errorf("missing page status = %d, want 404", got[2].Status)
	}
}

func TestRetriesTransientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail the first HEAD and GET, then succeed.
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	c := NewChecker(1, time.Second)
	c.RetryWait = time.Millisecond
	if r := c.Check(context.Background(), srv.URL); r.Broken() {
		t.Fatalf("expected success after retry, got status %d err %q", r.Status, r.Err)
	}

	c.Retries = 0
	calls.Store(0)
	if r := c.Check(context.Background(), srv.URL); !r.Broken() {
		t.Fatal("expected failure with retries disabled")
	}
}

func TestNoRetryOn404(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewChecker(1, time.Second)
	c.RetryWait = time.Millisecond
	c.Check(context.Background(), srv.URL)
	if n := calls.Load(); n != 2 { // one HEAD and one GET, no retries
		t.Fatalf("got %d requests, want 2", n)
	}
}
