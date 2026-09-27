# Observability

The API exports OpenTelemetry metrics through the Prometheus exporter and exposes them at /metrics.

Tracing uses OpenTelemetry. Set OTEL_EXPORTER_OTLP_ENDPOINT to enable OTLP/HTTP trace export; when unset, spans remain local to the process.

Prometheus and Grafana are included in the local Docker Compose stack.
