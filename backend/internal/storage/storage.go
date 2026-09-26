package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

const MaxPresignedURLExpiry = 15 * time.Minute

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
}

type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

type PresignedURL struct {
	URL       string
	ExpiresAt time.Time
}

type StorageProvider interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, options PutOptions) error
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (PresignedURL, error)
	PresignPut(ctx context.Context, key string, expiry time.Duration, contentType string) (PresignedURL, error)
}

func validateKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("storage key must not be empty")
	}
	if strings.ContainsAny(key, "\\") || path.IsAbs(key) || path.Clean(key) != key {
		return fmt.Errorf("invalid storage key")
	}
	for _, part := range strings.Split(key, "/") {
		if part == ".." || part == "." || part == "" {
			return fmt.Errorf("invalid storage key")
		}
	}
	return nil
}

func validateExpiry(expiry time.Duration) error {
	if expiry <= 0 {
		return fmt.Errorf("presigned URL expiry must be positive")
	}
	if expiry > MaxPresignedURLExpiry {
		return fmt.Errorf("presigned URL expiry exceeds maximum of %s", MaxPresignedURLExpiry)
	}
	return nil
}
