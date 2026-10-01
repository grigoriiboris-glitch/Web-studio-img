FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app \
    && mkdir -p /data/assets \
    && chown -R app:app /data/assets
WORKDIR /app
COPY --from=build /out/api /app/api
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
