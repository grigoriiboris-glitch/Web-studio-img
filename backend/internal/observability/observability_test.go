package observability

import (
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestNewAPIMetrics(t *testing.T) {
	mp := sdkmetric.NewMeterProvider()
	if _, err := NewAPIMetrics(mp); err != nil {
		t.Fatal(err)
	}
}

func TestNewJobMetrics(t *testing.T) {
	mp := sdkmetric.NewMeterProvider()
	if _, err := NewJobMetrics(mp); err != nil {
		t.Fatal(err)
	}
	_ = otel.GetMeterProvider()
}
