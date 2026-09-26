package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"web-studio-img/backend/internal/observability"
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
	server := NewServer(slog.Default(), []string{"http://localhost:5173"}, metrics)
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
