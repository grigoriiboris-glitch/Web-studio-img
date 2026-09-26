package observability

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

type Config struct { ServiceName string }

type Providers struct {
	MeterProvider *sdkmetric.MeterProvider
	TracerProvider *trace.TracerProvider
}

func Setup(ctx context.Context, cfg Config) (*Providers, error) {
	if cfg.ServiceName == "" { return nil, errors.New("service name is required") }
	metricExporter, err := prometheus.New()
	if err != nil { return nil, err }
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricExporter))
	tracerProvider := trace.NewTracerProvider()
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		exporter, err := otlptracehttp.New(ctx)
		if err != nil { return nil, err }
		tracerProvider = trace.NewTracerProvider(trace.WithBatcher(exporter))
	}
	otel.SetMeterProvider(meterProvider)
	otel.SetTracerProvider(tracerProvider)
	return &Providers{MeterProvider: meterProvider, TracerProvider: tracerProvider}, nil
}

func (p *Providers) Shutdown(ctx context.Context) error {
	if p == nil { return nil }
	return errors.Join(p.TracerProvider.Shutdown(ctx), p.MeterProvider.Shutdown(ctx))
}

type APIMetrics struct {
	Requests otelmetric.Int64Counter
	Errors otelmetric.Int64Counter
	Latency otelmetric.Float64Histogram
}

func NewAPIMetrics(mp *sdkmetric.MeterProvider) (APIMetrics, error) {
	meter := mp.Meter("web-studio-img/http")
	requests, err := meter.Int64Counter("api_requests_total")
	if err != nil { return APIMetrics{}, err }
	apiErrors, err := meter.Int64Counter("api_errors_total")
	if err != nil { return APIMetrics{}, err }
	latency, err := meter.Float64Histogram("api_latency_seconds")
	if err != nil { return APIMetrics{}, err }
	return APIMetrics{Requests: requests, Errors: apiErrors, Latency: latency}, nil
}

func DurationSeconds(start time.Time) float64 { return time.Since(start).Seconds() }
