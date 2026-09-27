package prompts

import "testing"

func TestBuildFinalTextUsesComponentOrder(t *testing.T) {
	got := BuildFinalText(map[string]string{
		"texture": "fine grain",
		"subject": "red chair",
		"composition": "centered",
		"negative_constraints": "no people",
	})
	want := "red chair, centered, fine grain, negative constraints: no people"
	if got != want { t.Fatalf("got %q, want %q", got, want) }
}

func TestValidateRejectsUnknownComponent(t *testing.T) {
	req := Request{OriginalText: "chair", Components: map[string]string{"unknown": "x"}, CreatedBy: CreatedByHuman}
	if err := req.Validate(); err == nil { t.Fatal("expected unknown component to be rejected") }
}
