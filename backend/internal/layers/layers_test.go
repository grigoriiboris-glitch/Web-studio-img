package layers

import "testing"

func TestLayerValidation(t *testing.T) {
	validTypes := []string{"base", "mask", "image", "manual_edit", "adjustment", "group"}
	for _, value := range validTypes {
		if !validLayerType(value) {
			t.Fatalf("expected valid layer type %q", value)
		}
	}
	for _, value := range []string{"", "unknown", "Layer"} {
		if validLayerType(value) {
			t.Fatalf("expected invalid layer type %q", value)
		}
	}

	validSources := []string{"human", "ai", "derived", "imported"}
	for _, value := range validSources {
		if !validSourceKind(value) {
			t.Fatalf("expected valid source kind %q", value)
		}
	}
	for _, value := range []string{"", "provider", "external"} {
		if validSourceKind(value) {
			t.Fatalf("expected invalid source kind %q", value)
		}
	}
}
