package httpapi

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

func newEmbeddedStaticHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if _, err := fs.Stat(root, name); err != nil {
			name = "index.html"
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + name
		fileServer.ServeHTTP(w, r2)
	})
}

func newStaticHandler(root string) http.Handler {
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		cleanPath := path.Clean("/" + r.URL.Path)
		localPath := path.Join(root, cleanPath)
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			r2 := r.Clone(r.Context())
			r2.URL.Path = cleanPath
			fileServer.ServeHTTP(w, r2)
			return
		}
		index := path.Join(root, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}
