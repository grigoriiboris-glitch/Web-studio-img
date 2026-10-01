package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticHandlerServesFilesAndSPA(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>app</html>"), 0o644); err != nil { t.Fatal(err) }
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil { t.Fatal(err) }

	handler := newStaticHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log('ok')" { t.Fatalf("asset response: status=%d body=%q", rec.Code, rec.Body.String()) }

	req = httptest.NewRequest(http.MethodGet, "/projects/demo/studio", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "<html>app</html>" { t.Fatalf("SPA response: status=%d body=%q", rec.Code, rec.Body.String()) }

	req = httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound { t.Fatalf("API fallback status=%d", rec.Code) }
}
