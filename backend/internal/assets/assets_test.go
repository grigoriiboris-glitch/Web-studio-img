package assets

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

	"github.com/google/uuid"
)

type fakeScanner struct {
	called bool
	err    error
}

func (s *fakeScanner) Scan(context.Context, []byte, string) error {
	s.called = true
	return s.err
}

func TestStorageKeysMatchSpecification(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	projectID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	assetID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	original, preview, thumbnail := storageKeys(userID, projectID, assetID)
	wantPrefix := "users/" + userID.String() + "/projects/" + projectID.String() + "/assets/" + assetID.String()
	if original != wantPrefix+"/original" || preview != wantPrefix+"/preview" || thumbnail != wantPrefix+"/thumbnail" {
		t.Fatalf("unexpected keys: %q %q %q", original, preview, thumbnail)
	}
}

func TestNormalizeImageByReencoding(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var source bytes.Buffer
	if err := png.Encode(&source, img); err != nil { t.Fatal(err) }
	normalized, err := normalizeImage(source.Bytes(), "image/png")
	if err != nil { t.Fatal(err) }
	if _, _, err := image.DecodeConfig(bytes.NewReader(normalized)); err != nil { t.Fatal(err) }
}

func TestProcessorUsesInjectedScanner(t *testing.T) {
	scanner := &fakeScanner{}
	processor := Processor{Scanner: scanner}
	if err := processor.scanner().Scan(context.Background(), []byte("data"), "image/png"); err != nil { t.Fatal(err) }
	if !scanner.called { t.Fatal("expected injected scanner to be called") }
}
