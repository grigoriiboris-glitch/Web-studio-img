package httpapi

import (
	"reflect"
	"io/fs"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
	"github.com/oleg3190/Web-studio-img/backend/internal/iterations"
	"github.com/oleg3190/Web-studio-img/backend/internal/observability"
	"github.com/oleg3190/Web-studio-img/backend/internal/projects"
	"github.com/oleg3190/Web-studio-img/backend/internal/security"
)

type Registrar interface {
	Register(*http.ServeMux)
}

type Server struct {
	handler http.Handler
}

func NewServer(logger *slog.Logger, origins []string, limiter *security.RateLimiter) *Server {
	return newServer(logger, origins, limiter, nil, nil, nil, nil, "")
}

func NewServerWithProjects(logger *slog.Logger, origins []string, limiter *security.RateLimiter, projectHandler *projects.Handler) *Server {
	return newServer(logger, origins, limiter, projectHandler, nil, nil, nil, "")
}

func NewServerWithProjectsAndIterations(logger *slog.Logger, origins []string, limiter *security.RateLimiter, projectHandler *projects.Handler, iterationHandler *iterations.Handler) *Server {
	return newServer(logger, origins, limiter, projectHandler, iterationHandler, nil, nil, "")
}

func NewServerWithProjectsIterationsAndGeneration(logger *slog.Logger, origins []string, limiter *security.RateLimiter, projectHandler *projects.Handler, iterationHandler *iterations.Handler, generationHandler *generation.Handler) *Server {
	return newServer(logger, origins, limiter, projectHandler, iterationHandler, generationHandler, nil, "")
}

func newServer(
	logger *slog.Logger,
	origins []string,
	limiter *security.RateLimiter,
	projectHandler *projects.Handler,
	iterationHandler *iterations.Handler,
	generationHandler *generation.Handler,
	metrics *observability.APIMetrics,
	staticDir string,
	registrars ...Registrar,
) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler)
	mux.HandleFunc("GET /api/v1/health", apiHealthHandler)

	if projectHandler != nil {
		projectHandler.Register(mux)
	}
	if iterationHandler != nil {
		iterationHandler.Register(mux)
	}
	if generationHandler != nil {
		generationHandler.Register(mux)
	}
	for _, registrar := range registrars {
		if !isNilRegistrar(registrar) {
			registrar.Register(mux)
		}
	}
	if staticDir != "" {
		mux.Handle("/", newStaticHandler(staticDir))
	}

	var handler http.Handler = mux
	handler = withCORS(origins, handler)
	handler = withSecurityHeaders(handler)
	handler = withRequestID(handler)
	handler = withRateLimit(limiter, handler)
	handler = withLogging(logger, handler)
	handler = http.MaxBytesHandler(handler, security.DefaultMaxUploadSize)
	if metrics != nil {
		handler = withObservability(*metrics, handler)
	}

	return &Server{handler: handler}
}

func NewServerWithStudioAndObservabilityAndEmbeddedStatic(logger *slog.Logger, origins []string, limiter *security.RateLimiter, metrics observability.APIMetrics, staticFS fs.FS, registrars ...Registrar) *Server {
	return newServerWithEmbeddedStatic(logger, origins, limiter, &metrics, staticFS, nil, nil, registrars...)
}

func NewServerWithStudioAndObservabilityAndEmbeddedStaticAndAuth(logger *slog.Logger, origins []string, limiter *security.RateLimiter, metrics observability.APIMetrics, staticFS fs.FS, tokenManager *auth.TokenManager, authStore auth.Store, registrars ...Registrar) *Server {
	return newServerWithEmbeddedStatic(logger, origins, limiter, &metrics, staticFS, tokenManager, authStore, registrars...)
}

func newServerWithEmbeddedStatic(logger *slog.Logger, origins []string, limiter *security.RateLimiter, metrics *observability.APIMetrics, staticFS fs.FS, tokenManager *auth.TokenManager, authStore auth.Store, registrars ...Registrar) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler)
	mux.HandleFunc("GET /api/v1/health", apiHealthHandler)
	for _, registrar := range registrars {
		if !isNilRegistrar(registrar) { registrar.Register(mux) }
	}
	mux.Handle("/", newEmbeddedStaticHandler(staticFS))
	var handler http.Handler = mux
	handler = withCORS(origins, handler)
	handler = withSecurityHeaders(handler)
	handler = withRequestID(handler)
	handler = withRateLimit(limiter, handler)
	handler = withLogging(logger, handler)
	handler = http.MaxBytesHandler(handler, security.DefaultMaxUploadSize)
	if tokenManager != nil && authStore != nil {
		handler = withAPIAuth(tokenManager, authStore, handler)
	}
	if metrics != nil { handler = withObservability(*metrics, handler) }
	return &Server{handler: handler}
}

func isNilRegistrar(registrar Registrar) bool {
	if registrar == nil { return true }
	value := reflect.ValueOf(registrar)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) HTTPServer(addr string, readTimeout, writeTimeout, idleTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      s.handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func apiHealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "web-studio-img-api"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func NewServerWithStudio(logger *slog.Logger, origins []string, limiter *security.RateLimiter, registrars ...Registrar) *Server {
	return newServer(logger, origins, limiter, nil, nil, nil, nil, "", registrars...)
}

func NewServerWithStudioAndObservability(logger *slog.Logger, origins []string, limiter *security.RateLimiter, metrics observability.APIMetrics, registrars ...Registrar) *Server {
	return newServer(logger, origins, limiter, nil, nil, nil, &metrics, "", registrars...)
}

func NewServerWithStudioAndObservabilityAndStatic(logger *slog.Logger, origins []string, limiter *security.RateLimiter, metrics observability.APIMetrics, staticDir string, registrars ...Registrar) *Server {
	return newServer(logger, origins, limiter, nil, nil, nil, &metrics, staticDir, registrars...)
}
