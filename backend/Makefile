DB_URL ?= postgres://webstudio:webstudio@localhost:5432/webstudio?sslmode=disable
SQLC_VERSION ?= v1.29.0

.PHONY: sqlc test fmt vet migrate-up migrate-down

sqlc:
	sqlc generate

test:
	go test ./...

fmt:
	gofmt -w ./internal ./cmd

vet:
	go vet ./...

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1
