package generation

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateRequestRequiresIdempotencyAndPrompt(t *testing.T) {
	req := Request{ProjectID: uuid.New(), Prompt: "prompt"}
	if err := ValidateRequest(req); err == nil {
		t.Fatal("expected missing idempotency key to fail")
	}
	req.IdempotencyKey = "key"
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRequestRejectsOversizedPrompt(t *testing.T) {
	req := Request{ProjectID: uuid.New(), Prompt: string(make([]byte, 20001)), IdempotencyKey: "key"}
	if err := ValidateRequest(req); err == nil {
		t.Fatal("expected oversized prompt to fail")
	}
}

func TestValidateRequestRejectsDuplicateReferenceIDs(t *testing.T) {
	id := uuid.New()
	req := Request{ProjectID: uuid.New(), Prompt: "test", IdempotencyKey: "key", ReferenceIDs: []uuid.UUID{id, id}}
	if err := ValidateRequest(req); err != ErrInvalidGeneration {
		t.Fatalf("expected duplicate reference IDs to be rejected, got %v", err)
	}
}

func TestValidateRequestAcceptsReproducibilityMetadata(t *testing.T) {
	req := Request{ProjectID: uuid.New(), Prompt: "test", IdempotencyKey: "key", ReferenceIDs: []uuid.UUID{uuid.New()}}
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("expected valid reference metadata, got %v", err)
	}
}

func TestValidateRequestRequiresRecipeVersion(t *testing.T) {
	id := uuid.New()
	req := Request{ProjectID: uuid.New(), Prompt: "test", IdempotencyKey: "key", RecipeID: &id}
	if err := ValidateRequest(req); err != ErrInvalidGeneration {
		t.Fatalf("expected recipe version to be required, got %v", err)
	}
	req.RecipeVersion = 2
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("expected explicit recipe version to be accepted, got %v", err)
	}
}
