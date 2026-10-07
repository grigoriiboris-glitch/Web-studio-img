package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"testing/fstest"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
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


func TestTypedNilRegistrarIsIgnored(t *testing.T) {
	var authHandler *auth.HTTPHandler
	server := NewServerWithStudio(slog.Default(), nil, nil, authHandler)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"a@example.com","name":"A","password":"StrongPassword123!"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecoveryTurnsHandlerPanicIntoInternalServerError(t *testing.T) {
	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	server := &Server{handler: withRecovery(slog.Default(), withRequestID(panicHandler))}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("missing error body: %v", body)
	}
	if got := rec.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("missing request id")
	}
}

type routeAuthStore struct {
	user auth.User
	session uuid.UUID
}

func (s *routeAuthStore) FindUserByEmail(context.Context, string) (auth.User, error) { return s.user, nil }
func (s *routeAuthStore) CreateUser(_ context.Context, email, name, passwordHash string) (auth.User, error) {
	s.user = auth.User{ID: uuid.New(), Email: email, Name: name, PasswordHash: passwordHash}
	return s.user, nil
}
func (s *routeAuthStore) CreateSession(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (uuid.UUID, error) {
	s.session = uuid.New()
	return s.session, nil
}
func (s *routeAuthStore) IsSessionActive(context.Context, uuid.UUID, time.Time) (bool, error) { return true, nil }
func (s *routeAuthStore) RevokeSession(context.Context, uuid.UUID, time.Time) error { return nil }

func TestAuthRegisterRealServerRoute(t *testing.T) {
	store := &routeAuthStore{}
	tokens, err := auth.NewTokenManager("12345678901234567890123456789012", "web-studio-img", time.Hour)
	if err != nil { t.Fatal(err) }
	service, err := auth.NewService(store, tokens)
	if err != nil { t.Fatal(err) }
	handler, err := auth.NewHTTPHandler(service, tokens, store)
	if err != nil { t.Fatal(err) }
	mp := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })
	metrics, err := observability.NewAPIMetrics(mp)
	if err != nil { t.Fatal(err) }
	server := NewServerWithStudioAndObservabilityAndEmbeddedStaticAndAuth(
		slog.Default(), []string{"http://localhost:5173"}, nil,
		metrics, fstest.MapFS{}, tokens, store, handler,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"artist@example.com","name":"Test Artist","password":"StrongPassword123!"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated { t.Fatalf("registration route status=%d body=%s", rec.Code, rec.Body.String()) }
	var body struct {
		User struct { Email string `json:"email"`; Name string `json:"name"` } `json:"user"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil { t.Fatalf("decode response: %v", err) }
	if body.User.Email != "artist@example.com" || body.User.Name != "Test Artist" || body.Token == "" { t.Fatalf("unexpected registration response: %+v", body) }
}
