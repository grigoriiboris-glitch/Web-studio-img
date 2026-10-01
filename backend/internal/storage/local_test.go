package storage

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLocalStoragePutGetDelete(t *testing.T) {
	store, err := NewLocalStorage(LocalConfig{
		RootDir: filepath.Join(t.TempDir(), "assets"),
		SigningSecret: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("hello local storage")
	ctx := context.Background()

	if err := store.Put(ctx, "users/u/projects/p/assets/a/original", bytes.NewReader(body), int64(len(body)), PutOptions{
		ContentType: "image/png",
		Metadata: map[string]string{"sha256": "abc"},
	}); err != nil {
		t.Fatal(err)
	}

	reader, info, err := store.Get(ctx, "users/u/projects/p/assets/a/original")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body=%q, want %q", got, body)
	}
	if info.Size != int64(len(body)) || info.ContentType != "image/png" || info.ETag == "" {
		t.Fatalf("unexpected object info: %+v", info)
	}

	if err := store.Delete(ctx, "users/u/projects/p/assets/a/original"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(ctx, "users/u/projects/p/assets/a/original"); err == nil {
		t.Fatal("expected deleted object to be missing")
	}
}

func TestLocalStoragePresignedURLs(t *testing.T) {
	store, err := NewLocalStorage(LocalConfig{RootDir: t.TempDir(), SigningSecret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := store.PresignPut(context.Background(), "users/u/object", time.Minute, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	expiry, err := strconv.ParseInt(query.Get("expires"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !store.ValidatePresignedURL("PUT", query.Get("key"), expiry, query.Get("sig")) {
		t.Fatal("expected valid presigned PUT URL")
	}
	if store.ValidatePresignedURL("GET", query.Get("key"), expiry, query.Get("sig")) {
		t.Fatal("PUT signature must not validate for GET")
	}
}

func TestLocalStorageRejectsUnsafeKeys(t *testing.T) {
	store, err := NewLocalStorage(LocalConfig{RootDir: t.TempDir(), SigningSecret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../secret", "/absolute", "users/u/../../secret"} {
		if err := store.Put(context.Background(), key, strings.NewReader("x"), 1, PutOptions{}); err == nil {
			t.Fatalf("Put(%q) expected error", key)
		}
	}
}

func TestLocalStorageRootIsCreated(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "assets")
	if _, err := NewLocalStorage(LocalConfig{RootDir: root, SigningSecret: "test-secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("local storage root is not a directory")
	}
}
