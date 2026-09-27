package events

import (
	"bytes"
	"testing"
)

func TestEncodePayloadCanonicalJSON(t *testing.T) {
	got, err := EncodePayload(map[string]any{"b": 2, "a": 1})
	if err != nil { t.Fatalf("EncodePayload: %v", err) }
	want := []byte(`{"a":1,"b":2}`)
	if !bytes.Equal(got, want) { t.Fatalf("got %s, want %s", got, want) }
}
