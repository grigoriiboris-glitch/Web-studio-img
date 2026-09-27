package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oleg3190/Web-studio-img/backend/internal/observability"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestWithObservabilityRecordsRequests(t *testing.T) {
	mp := sdkmetric.NewMeterProvider()
	metrics, err := observability.NewAPIMetrics(mp)
	if err != nil { t.Fatal(err) }
	handler := withObservability(metrics, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/test", nil))
	if rec.Code != http.StatusCreated { t.Fatalf("status=%d", rec.Code) }
	if err := mp.Shutdown(t.Context()); err != nil { t.Fatal(err) }
}

func TestResponseRecorderDefaultsStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: rec}
	if _, err := rw.Write([]byte("ok")); err != nil { t.Fatal(err) }
	if rw.status != http.StatusOK { t.Fatalf("status=%d", rw.status) }
}
