package security

import (
	"bytes"
	"testing"
	"time"
)

func TestValidateImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n" + "test")
	data, detected, err := ValidateImage(bytes.NewReader(png), "image/png; charset=binary", 1024)
	if err != nil {
		t.Fatalf("ValidateImage() error = %v", err)
	}
	if len(data) != len(png) || detected != "image/png" {
		t.Fatalf("unexpected result: len=%d mime=%q", len(data), detected)
	}
}

func TestValidateImageRejectsMismatch(t *testing.T) {
	_, _, err := ValidateImage(bytes.NewReader([]byte("plain text")), "image/png", 1024)
	if err == nil {
		t.Fatal("expected MIME mismatch error")
	}
}

func TestValidateImageRejectsOversized(t *testing.T) {
	_, _, err := ValidateImage(bytes.NewReader([]byte("\x89PNG\r\n\x1a\n")), "image/png", 4)
	if err == nil {
		t.Fatal("expected size error")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	now := time.Unix(100, 0)
	if !limiter.Allow("client", now) || !limiter.Allow("client", now) {
		t.Fatal("expected first two requests to be allowed")
	}
	if limiter.Allow("client", now) {
		t.Fatal("expected third request to be rejected")
	}
	if !limiter.Allow("other", now) {
		t.Fatal("expected separate client to be allowed")
	}
	if !limiter.Allow("client", now.Add(time.Minute)) {
		t.Fatal("expected request after window to be allowed")
	}
}

func TestAllowedImageMIME(t *testing.T) {
	for _, mimeType := range []string{"image/jpeg", "image/png", "image/webp"} {
		if !IsAllowedImageMIME(mimeType) {
			t.Fatalf("expected %s to be allowed", mimeType)
		}
	}
	if IsAllowedImageMIME("image/svg+xml") {
		t.Fatal("SVG must not be accepted as an image upload")
	}
}
