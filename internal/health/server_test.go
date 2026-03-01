package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHealthz_AlwaysOK(t *testing.T) {
	srv := NewServer(0, func() bool { return false }, testLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", srv.handleHealthz)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestReadyz_NotReady(t *testing.T) {
	srv := NewServer(0, func() bool { return false }, testLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", srv.handleReadyz)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "not ready" {
		t.Errorf("expected status 'not ready', got %q", resp.Status)
	}
}

func TestReadyz_Ready(t *testing.T) {
	srv := NewServer(0, func() bool { return true }, testLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", srv.handleReadyz)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", resp.Status)
	}
}

func TestReadyz_NilCallback(t *testing.T) {
	srv := NewServer(0, nil, testLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", srv.handleReadyz)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with nil callback, got %d", w.Code)
	}
}

func TestReadyz_StateTransition(t *testing.T) {
	var ready atomic.Bool

	srv := NewServer(0, ready.Load, testLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", srv.handleReadyz)

	// Initially not ready.
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 initially, got %d", w.Code)
	}

	// Transition to ready.
	ready.Store(true)

	req = httptest.NewRequest("GET", "/readyz", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after ready, got %d", w.Code)
	}
}

func TestHealthz_ContentType(t *testing.T) {
	srv := NewServer(0, func() bool { return true }, testLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", srv.handleHealthz)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
