package rejectreasons

import "testing"

func TestNormalize(t *testing.T) {
	got, err := Normalize([]string{" Lighting ", "composition"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"composition", "lighting"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizeRejectsUnknownAndDuplicate(t *testing.T) {
	if _, err := Normalize([]string{"camera"}); err == nil {
		t.Fatal("expected unknown reason error")
	}
	if _, err := Normalize([]string{"color", "Color"}); err == nil {
		t.Fatal("expected duplicate reason error")
	}
}
