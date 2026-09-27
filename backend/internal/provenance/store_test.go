package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func TestHashEventUsesCanonicalPayloadPlusParentHash(t *testing.T) {
	parent := "parent-hash"
	event := Event{
		EntityType: "generation",
		EntityID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Action: "generation.completed",
		Payload: map[string]any{"b": 2, "a": 1},
		ParentHash: parent,
	}
	got, err := HashEvent(event)
	if err != nil { t.Fatalf("HashEvent: %v", err) }
	sum := sha256.Sum256([]byte(`{"a":1,"b":2}` + parent))
	want := hex.EncodeToString(sum[:])
	if got != want { t.Fatalf("got %q, want %q", got, want) }
}

func TestCanonicalPayloadDeterministic(t *testing.T) {
	a, err := CanonicalPayload(map[string]any{"z": 1, "a": 2})
	if err != nil { t.Fatal(err) }
	b, err := CanonicalPayload(map[string]any{"a": 2, "z": 1})
	if err != nil { t.Fatal(err) }
	if string(a) != string(b) { t.Fatalf("payload is not deterministic: %s != %s", a, b) }
}
