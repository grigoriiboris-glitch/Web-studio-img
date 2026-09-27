package events

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

func TestParseLastEventID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
		ok    bool
	}{
		{name: "empty", input: "", want: 0, ok: true},
		{name: "valid", input: "42", want: 42, ok: true},
		{name: "zero", input: "0", want: 0, ok: true},
		{name: "negative", input: "-1", want: 0, ok: false},
		{name: "invalid", input: "abc", want: 0, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLastEventID(tt.input)
			if tt.ok && err != nil { t.Fatalf("unexpected error: %v", err) }
			if !tt.ok && err == nil { t.Fatal("expected error") }
			if got != tt.want { t.Fatalf("got %d, want %d", got, tt.want) }
		})
	}
}

type fakeEventReader struct {
	mu      sync.Mutex
	after   int64
	queried chan struct{}
	once    bool
}

func (f *fakeEventReader) ProjectOwned(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakeEventReader) ListSince(ctx context.Context, _ uuid.UUID, _ uuid.UUID, after int64, _ int) ([]Event, error) {
	f.mu.Lock()
	f.after = after
	if !f.once {
		f.once = true
		select { case f.queried <- struct{}{}: default: }
		f.mu.Unlock()
		return []Event{{ID: 42, ProjectID: uuid.New(), UserID: uuid.New(), EventType: "generation.completed", EntityType: "generation", EntityID: uuid.New()}}, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestStreamReplaysFromLastEventID(t *testing.T) {
	store := &fakeEventReader{queried: make(chan struct{}, 1)}
	handler, err := NewHandler(store)
	if err != nil { t.Fatal(err) }
	ctx, cancel := context.WithCancel(auth.WithPrincipal(context.Background(), auth.Principal{UserID: uuid.New()}))
	defer cancel()
	req := httptest.NewRequest("GET", "/api/v1/projects/"+uuid.NewString()+"/events", nil).WithContext(ctx)
	req.Header.Set("Last-Event-ID", "41")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.stream(rec, req)
		close(done)
	}()
	select {
	case <-store.queried:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not query event store")
	}
	body := rec.Body.String()
	if store.after != 41 { t.Fatalf("store queried after=%d, want 41", store.after) }
	if !strings.Contains(body, "id: 42\n") { t.Fatalf("missing SSE id: %s", body) }
	if !strings.Contains(body, "event: generation.completed\n") { t.Fatalf("missing SSE event: %s", body) }
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not stop after context cancellation")
	}
}
