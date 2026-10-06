package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/security"
)

type requestIDKey struct{}

func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestID := requestIDFromContext(r.Context())
				logger.Error("http panic recovered", "panic", recovered, "request_id", requestID, "method", r.Method, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "internal_error", "message": "internal server error", "request_id": requestID}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path,
			"duration", time.Since(start), "request_id", requestIDFromContext(r.Context()))
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func withCORS(origins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		_, ok := allowed[origin]
		if ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Idempotency-Key, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			if origin != "" && !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withRateLimit(limiter *security.RateLimiter, next http.Handler) http.Handler {
	if limiter == nil {
		return next
	}
	return limiter.Middleware(next)
}


func withAPIAuth(manager *auth.TokenManager, store auth.Store, next http.Handler) http.Handler {
    protected := auth.WithAuth(manager, store, next)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodOptions || !strings.HasPrefix(r.URL.Path, "/api/v1/") {
            next.ServeHTTP(w, r)
            return
        }
        switch r.URL.Path {
        case "/api/v1/health", "/api/v1/auth/login", "/api/v1/auth/register":
            next.ServeHTTP(w, r)
            return
        default:
            protected.ServeHTTP(w, r)
        }
    })
}
