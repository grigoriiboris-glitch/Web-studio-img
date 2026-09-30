package references

import (
	"testing"

	"github.com/google/uuid"
)

func TestUsageRequestRequiresCreativeTarget(t *testing.T) {
	req := UsageRequest{UsageType: "influence"}
	if err := req.Validate(); err != ErrInvalidUsage {
		t.Fatalf("expected invalid usage, got %v", err)
	}
}

func TestUsageRequestAcceptsIterationAndInfluence(t *testing.T) {
	id := uuid.New()
	req := UsageRequest{
		IterationID: &id,
		UsageType: "generation",
		Role: "composition",
		Influence: &Influence{Composition: 0.8, Semantic: 0.4, Color: 0.2, Style: 0.3, Material: 0.1, Geometry: 0.6, Mood: 0.5},
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid usage, got %v", err)
	}
	if req.Role != "composition" { t.Fatalf("expected composition role, got %q", req.Role) }
}
