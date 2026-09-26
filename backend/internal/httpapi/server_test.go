package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/security"
)

func TestHealthEndpoint(t *testing.T) {
	server := NewServer(slog.Default(), []string{"http://localhost:5173"}, security.NewRateLimiter(10, time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security header missing")
	}
}

func TestRateLimit(t *testing.T) {
	server := NewServer(slog.Default(), nil, security.NewRateLimiter(1, time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d", rec.Code)
	}
}
