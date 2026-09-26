module github.com/oleg3190/Web-studio-img/backend

go 1.24

require (
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.29.8
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.64
	github.com/aws/aws-sdk-go-v2/service/s3 v1.79.1
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/hibiken/asynq v0.25.1
	golang.org/x/crypto v0.41.0
	go.opentelemetry.io/otel v1.34.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.34.0
	go.opentelemetry.io/otel/exporters/prometheus v0.56.0
	go.opentelemetry.io/otel/sdk v1.34.0
	github.com/prometheus/client_golang v1.21.1
)
