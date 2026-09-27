package httpapi

import (
	"net/http"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

type responseRecorder struct { http.ResponseWriter; status int }

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 { w.status = http.StatusOK }
	return w.ResponseWriter.Write(body)
}

func withObservability(metrics observability.APIMetrics, next http.Handler) http.Handler {
	tracer := otel.Tracer("web-studio-img/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		r = r.WithContext(ctx)
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 { status = http.StatusOK }
		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", r.URL.Path),
			attribute.Int("http.response.status_code", status),
		}
		opt := metric.WithAttributes(attrs...)
		metrics.Requests.Add(r.Context(), 1, opt)
		metrics.Latency.Record(r.Context(), time.Since(start).Seconds(), opt)
		if status >= 500 {
			metrics.Errors.Add(r.Context(), 1, opt)
			span.SetStatus(codes.Error, http.StatusText(status))
		}
		span.SetAttributes(attrs...)
	})
}
