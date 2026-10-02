package variants

import (
  "strings"
  "testing"

  "github.com/google/uuid"
)

func TestRegenerationGenerationKeyIsBoundedAndStable(t *testing.T) {
  setID, variantID := uuid.New(), uuid.New()
  key := strings.Repeat("x", 200)
  first := regenerationGenerationKey(setID, variantID, key)
  second := regenerationGenerationKey(setID, variantID, key)
  if first != second {
    t.Fatal("regeneration generation key is not stable")
  }
  if len(first) > 200 {
    t.Fatalf("regeneration generation key exceeds generation limit: %d", len(first))
  }
  if first == regenerationGenerationKey(setID, variantID, "different") {
    t.Fatal("different idempotency keys produced the same regeneration key")
  }
}
