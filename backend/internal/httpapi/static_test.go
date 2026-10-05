package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
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

func TestEmbeddedStaticHandlerDoesNotRedirectIndex(t *testing.T) {
	root := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>embedded</html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('embedded')")},
	}
	handler := newEmbeddedStaticHandler(root)

	const expectedCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:; manifest-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
	for _, path := range []string{"/", "/index.html", "/projects/demo/studio"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d location=%q", path, rec.Code, rec.Header().Get("Location"))
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != expectedCSP {
			t.Fatalf("%s: CSP=%q", path, got)
		}
		if got := rec.Header().Get("Location"); got != "" {
			t.Fatalf("%s: unexpected redirect to %q", path, got)
		}
		if rec.Body.String() != "<html>embedded</html>" {
			t.Fatalf("%s: body=%q", path, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log('embedded')" {
		t.Fatalf("asset response: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
