package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

)

func TestWriteError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey{}, "req-123"))
	rec := httptest.NewRecorder()

	writeError(rec, req, http.StatusBadRequest, "invalid_request", "bad request")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "invalid_request" || response.Error.Message != "bad request" || response.Error.RequestID != "req-123" {
		t.Fatalf("unexpected error response: %+v", response)
	}

	if got := requestIDFromContext(req.Context()); got != "req-123" {
		t.Fatalf("expected request id from context, got %q", got)
	}

}
