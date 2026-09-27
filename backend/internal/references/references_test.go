package references

import "testing"

func TestInfluenceWarning(t *testing.T) {
	req := Request{
		SourceType: SourceReference,
		License: "unknown",
		SourceURL: stringPtr("https://example.com/ref"),
		Influence: &Influence{Composition: 0.9},
	}
	if err := req.Validate(); err != nil { t.Fatal(err) }
	influence := NormalizeInfluence(req.Influence)
	if influence.Warning == "" { t.Fatal("expected composition warning") }
}

func TestInfluenceRangeValidation(t *testing.T) {
	req := Request{
		SourceType: SourceReference,
		License: "unknown",
		SourceURL: stringPtr("https://example.com/ref"),
		Influence: &Influence{Semantic: 1.1},
	}
	if err := req.Validate(); err == nil { t.Fatal("expected score range error") }
}

func stringPtr(value string) *string { return &value }
