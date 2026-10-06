package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/oleg3190/Web-studio-img/backend/internal/observability"
	"github.com/oleg3190/Web-studio-img/backend/internal/security"
)

func TestHealthEndpoint(t *testing.T) {
	mp := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })

	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	metrics, err := observability.NewAPIMetrics(mp)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServerWithStudioAndObservability(slog.Default(), []string{"http://localhost:5173"}, security.NewRateLimiter(10, time.Minute), metrics)
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

func TestAuthPreflightAllowsConfiguredFrontendOrigin(t *testing.T) {
	server := NewServer(slog.Default(), []string{"http://localhost:5173"}, nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/register", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status=%d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow-origin=%q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("allow-methods missing")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("allow-headers missing")
	}
}

func TestAuthPreflightRejectsUnknownOrigin(t *testing.T) {
	server := NewServer(slog.Default(), []string{"http://localhost:5173"}, nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/register", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("preflight status=%d", rec.Code)
	}
}
