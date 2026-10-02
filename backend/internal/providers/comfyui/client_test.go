package comfyui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientMatchesComfyUIHTTPContract(t *testing.T) {
	var uploaded bool
	var submitted bool
	var interrupted bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/upload/image":
			if err := r.ParseMultipartForm(2 << 20); err != nil {
				t.Fatalf("multipart parse: %v", err)
			}
			if _, _, err := r.FormFile("image"); err != nil {
				t.Fatalf("missing image field: %v", err)
			}
			if r.FormValue("type") != "input" || r.FormValue("overwrite") != "true" {
				t.Fatalf("unexpected upload fields: type=%q overwrite=%q", r.FormValue("type"), r.FormValue("overwrite"))
			}
			uploaded = true
			_ = json.NewEncoder(w).Encode(uploadResponse{Name: "input.png", Type: "input"})
		case r.Method == http.MethodPost && r.URL.Path == "/prompt":
			var req workflowRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode prompt: %v", err)
			}
			if req.Prompt["1"] == nil || req.ClientID == "" {
				t.Fatalf("unexpected prompt request: %#v", req)
			}
			submitted = true
			_ = json.NewEncoder(w).Encode(promptResponse{PromptID: "prompt-1"})
		case r.Method == http.MethodGet && r.URL.Path == "/history/prompt-1":
			_ = json.NewEncoder(w).Encode(map[string]historyEntry{
				"prompt-1": {
					Status: &historyStatus{StatusStr: "success", Completed: true},
					Outputs: map[string]nodeOutput{"3": {Images: []outputImage{{Filename: "result.png", Subfolder: "", Type: "output"}}}},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/view":
			if r.URL.Query().Get("filename") != "result.png" || r.URL.Query().Get("type") != "output" {
				t.Fatalf("unexpected view query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		case r.Method == http.MethodPost && r.URL.Path == "/interrupt":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["prompt_id"] != "prompt-1" {
				t.Fatalf("unexpected interrupt body: %#v", body)
			}
			interrupted = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{}")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newClient(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	upload, err := client.upload(context.Background(), "sketch.png", "image/png", []byte("sketch"))
	if err != nil || upload.Name != "input.png" || !uploaded {
		t.Fatalf("upload contract failed: upload=%#v err=%v uploaded=%v", upload, err, uploaded)
	}

	prompt, err := client.submit(context.Background(), map[string]any{"1": map[string]any{"class_type": "LoadImage"}}, "client-1")
	if err != nil || prompt.PromptID != "prompt-1" || !submitted {
		t.Fatalf("prompt contract failed: prompt=%#v err=%v submitted=%v", prompt, err, submitted)
	}

	history, err := client.history(context.Background(), "prompt-1")
	if err != nil || history.Status == nil || !history.Status.Completed {
		t.Fatalf("history contract failed: history=%#v err=%v", history, err)
	}

	data, mime, err := client.view(context.Background(), outputImage{Filename: "result.png", Type: "output"})
	if err != nil || mime != "image/png" || string(data) != "png-bytes" {
		t.Fatalf("view contract failed: mime=%q data=%q err=%v", mime, data, err)
	}

	if err := client.interrupt(context.Background(), "prompt-1"); err != nil || !interrupted {
		t.Fatalf("interrupt contract failed: err=%v interrupted=%v", err, interrupted)
	}
}

func TestClientRejectsComfyUINodeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"prompt_id":"prompt-1","node_errors":{"3":{"errors":["missing input"]}}}`)
	}))
	defer server.Close()

	client, err := newClient(server.URL, 5)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.submit(context.Background(), map[string]any{"3": map[string]any{}}, "client-1")
	if err == nil || !strings.Contains(err.Error(), "workflow node validation failed") {
		t.Fatalf("expected node validation error, got %v", err)
	}
}
