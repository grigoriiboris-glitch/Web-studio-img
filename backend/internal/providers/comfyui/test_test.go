package comfyui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
)

func TestProviderTestWorkflowDoesNotNeedGenerationStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/prompt" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"prompt_id":"smoke-1"}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/history/smoke-1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"smoke-1":{"status":{"status_str":"success","completed":true},"outputs":{}}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := newClient(server.URL, time.Second)
	if err != nil { t.Fatal(err) }
	provider := &Provider{client: client, cfg: Config{Timeout: time.Second, PollEvery: time.Millisecond}}

	result, err := provider.TestWorkflow(context.Background(), generation.Request{
		Prompt: "smoke test",
		Parameters: map[string]any{},
		ResolvedWorkflow: map[string]any{"1": map[string]any{"class_type":"LoadImage","inputs":map[string]any{}}},
		IdempotencyKey: "test-key",
	})
	if err != nil { t.Fatal(err) }
	if result.PromptID != "smoke-1" { t.Fatalf("prompt id=%q", result.PromptID) }
}
