package security

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

const DefaultMaxUploadSize int64 = 10 << 20

var allowedImageTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

func ValidateImage(r io.Reader, declaredMIME string, maxSize int64) ([]byte, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("upload body must not be nil")
	}
	if maxSize <= 0 {
		maxSize = DefaultMaxUploadSize
	}

	declaredMIME = normalizeMIME(declaredMIME)
	if _, ok := allowedImageTypes[declaredMIME]; !ok {
		return nil, "", fmt.Errorf("unsupported image MIME type %q", declaredMIME)
	}

	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("read upload: %w", err)
	}
	if int64(len(data)) > maxSize {
		return nil, "", fmt.Errorf("upload exceeds maximum size of %d bytes", maxSize)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("upload body must not be empty")
	}

	detected := normalizeMIME(http.DetectContentType(data))
	if detected != declaredMIME {
		return nil, "", fmt.Errorf("declared MIME %q does not match detected MIME %q", declaredMIME, detected)
	}
	return data, detected, nil
}

func normalizeMIME(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if mediaType, _, err := mime.ParseMediaType(value); err == nil {
		return mediaType
	}
	return value
}

func IsAllowedImageMIME(value string) bool {
	_, ok := allowedImageTypes[normalizeMIME(value)]
	return ok
}

func sniffPrefix(data []byte) []byte {
	return bytes.TrimSpace(data)
}
