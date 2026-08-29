package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"opora.local/api/internal/config"
)

type readinessStub struct{ err error }

func (s readinessStub) Ping(context.Context) error { return s.err }

func testConfig() config.HTTP {
	return config.HTTP{
		Address: ":0", ReadTimeout: time.Second, ReadHeaderTimeout: time.Second,
		WriteTimeout: time.Second, IdleTimeout: time.Second, ShutdownTimeout: time.Second,
	}
}

func TestLiveness(t *testing.T) {
	server := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), readinessStub{})
	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header is missing")
	}
}

func TestReadinessDoesNotLeakInternalError(t *testing.T) {
	server := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), readinessStub{err: errors.New("password=secret")})
	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(recorder.Body.String(), "password") || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatal("response leaked an internal error")
	}
}
