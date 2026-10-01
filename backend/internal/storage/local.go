package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const LocalStorageAPIPath = "/api/v1/storage/local"

type LocalConfig struct {
	RootDir       string
	SigningSecret string
}

type localMetadata struct {
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ETag        string            `json:"etag,omitempty"`
}

type LocalStorage struct {
	root   string
	secret []byte
	now    func() time.Time
}

func NewLocalStorage(cfg LocalConfig) (*LocalStorage, error) {
	root := strings.TrimSpace(cfg.RootDir)
	if root == "" {
		return nil, errors.New("local storage root directory must not be empty")
	}
	secret := strings.TrimSpace(cfg.SigningSecret)
	if secret == "" {
		return nil, errors.New("local storage signing secret must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local storage root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}
	return &LocalStorage{root: abs, secret: []byte(secret), now: time.Now}, nil
}

func (s *LocalStorage) Put(ctx context.Context, key string, body io.Reader, size int64, options PutOptions) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if body == nil {
		return errors.New("storage body must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	target, err := s.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create local storage directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return fmt.Errorf("create local storage temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer cleanup()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("set local storage permissions: %w", err)
	}
	written, err := io.Copy(tmp, body)
	if err != nil {
		return fmt.Errorf("write local object: %w", err)
	}
	if size >= 0 && written != size {
		return fmt.Errorf("local object size mismatch: got %d, want %d", written, size)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync local object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close local object: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("install local object: %w", err)
	}

	sum := sha256.New()
	reader, err := os.Open(target)
	if err != nil {
		_ = os.Remove(target)
		return fmt.Errorf("open local object for checksum: %w", err)
	}
	_, copyErr := io.Copy(sum, reader)
	closeErr := reader.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(target)
		if copyErr != nil {
			return fmt.Errorf("checksum local object: %w", copyErr)
		}
		return fmt.Errorf("close local object after checksum: %w", closeErr)
	}

	meta := localMetadata{
		ContentType: strings.TrimSpace(options.ContentType),
		Metadata:    options.Metadata,
		ETag:        hex.EncodeToString(sum.Sum(nil)),
	}
	if err := s.writeMetadata(target, meta); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if err := validateKey(key); err != nil {
		return nil, ObjectInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, ObjectInfo{}, err
	}
	target, err := s.pathForKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("get local object %q: %w", key, err)
	}
	if !info.Mode().IsRegular() {
		return nil, ObjectInfo{}, fmt.Errorf("local object %q is not a regular file", key)
	}

	meta := localMetadata{}
	if data, readErr := os.ReadFile(s.metadataPath(target)); readErr == nil {
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, ObjectInfo{}, fmt.Errorf("decode local object metadata: %w", err)
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return nil, ObjectInfo{}, fmt.Errorf("read local object metadata: %w", readErr)
	}

	file, err := os.Open(target)
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("open local object %q: %w", key, err)
	}
	return file, ObjectInfo{
		Key:         key,
		Size:        info.Size(),
		ContentType: meta.ContentType,
		ETag:        meta.ETag,
	}, nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := s.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete local object %q: %w", key, err)
	}
	if err := os.Remove(s.metadataPath(target)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete local object metadata %q: %w", key, err)
	}
	return nil
}

func (s *LocalStorage) PresignGet(ctx context.Context, key string, expiry time.Duration) (PresignedURL, error) {
	if err := validateKey(key); err != nil {
		return PresignedURL{}, err
	}
	if err := validateExpiry(expiry); err != nil {
		return PresignedURL{}, err
	}
	if err := ctx.Err(); err != nil {
		return PresignedURL{}, err
	}
	return s.presignedURL("GET", key, expiry), nil
}

func (s *LocalStorage) PresignPut(ctx context.Context, key string, expiry time.Duration, _ string) (PresignedURL, error) {
	if err := validateKey(key); err != nil {
		return PresignedURL{}, err
	}
	if err := validateExpiry(expiry); err != nil {
		return PresignedURL{}, err
	}
	if err := ctx.Err(); err != nil {
		return PresignedURL{}, err
	}
	return s.presignedURL("PUT", key, expiry), nil
}

func (s *LocalStorage) ValidatePresignedURL(method, key string, expiresAt int64, signature string) bool {
	if err := validateKey(key); err != nil || strings.TrimSpace(signature) == "" {
		return false
	}
	now := s.now()
	if expiresAt <= now.Unix() || expiresAt > now.Add(MaxPresignedURLExpiry+time.Minute).Unix() {
		return false
	}
	expected := signPayload(s.secret, signedPayload(method, key, expiresAt))
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(signature))), []byte(expected))
}

func (s *LocalStorage) presignedURL(method, key string, expiry time.Duration) PresignedURL {
	expiresAt := s.now().Add(expiry)
	values := url.Values{}
	values.Set("key", key)
	values.Set("expires", strconv.FormatInt(expiresAt.Unix(), 10))
	values.Set("sig", signPayload(s.secret, signedPayload(method, key, expiresAt.Unix())))
	return PresignedURL{
		URL:       LocalStorageAPIPath + "?" + values.Encode(),
		ExpiresAt: expiresAt,
	}
}

func signedPayload(method, key string, expiresAt int64) string {
	return strings.ToUpper(strings.TrimSpace(method)) + "\n" + key + "\n" + strconv.FormatInt(expiresAt, 10)
}

func signPayload(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *LocalStorage) pathForKey(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	target := filepath.Join(s.root, filepath.FromSlash(key))
	relative, err := filepath.Rel(s.root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", errors.New("invalid local storage path")
	}
	return target, nil
}

func (s *LocalStorage) metadataPath(target string) string {
	return target + ".meta.json"
}

func (s *LocalStorage) writeMetadata(target string, metadata localMetadata) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode local object metadata: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".meta-*")
	if err != nil {
		return fmt.Errorf("create local metadata temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer cleanup()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("set local metadata permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write local metadata: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync local metadata: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close local metadata: %w", err)
	}
	if err := os.Rename(tmpName, s.metadataPath(target)); err != nil {
		return fmt.Errorf("install local metadata: %w", err)
	}
	return nil
}
