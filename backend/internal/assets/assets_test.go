package assets

import (
	"context"
	"image"
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

func TestNormalizeImageStripsMetadataByReencoding(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	data, err := encodeTestPNG(img)
	if err != nil { t.Fatal(err) }
	normalized, err := normalizeImage(data, "image/png")
	if err != nil { t.Fatal(err) }
	if _, _, err := image.DecodeConfig(bytesReader(normalized)); err != nil { t.Fatal(err) }
}

func encodeTestPNG(img image.Image) ([]byte, error) {
	var buf bytesBuffer
	if err := pngEncode(&buf, img); err != nil { return nil, err }
	return buf.Bytes(), nil
}
