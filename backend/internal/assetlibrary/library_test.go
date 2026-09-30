package assetlibrary

import (
  "reflect"
  "testing"
)

func TestValidType(t *testing.T) {
  t.Parallel()
  if !validType("character") || !validType("image") {
    t.Fatal("expected supported asset types to be accepted")
  }
  if validType("video") || validType("") {
    t.Fatal("expected unsupported asset types to be rejected")
  }
}

func TestValidRights(t *testing.T) {
  t.Parallel()
  if !validRights("inherited") || !validRights("restricted") {
    t.Fatal("expected supported rights states to be accepted")
  }
  if validRights("licensed-unknown") {
    t.Fatal("expected unsupported rights state to be rejected")
  }
}

func TestNormalizeTags(t *testing.T) {
  t.Parallel()
  got := normalizeTags([]string{" Hero ", "hero", "Product", "", " PRODUCT "})
  want := []string{"hero", "product"}
  if !reflect.DeepEqual(got, want) {
    t.Fatalf("normalizeTags() = %#v, want %#v", got, want)
  }
}
