package humanactions

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateAllowsExplicitCreativeActions(t *testing.T) {
	projectID, userID := uuid.New(), uuid.New()
	for _, action := range []string{"IDEA_CREATED","PROMPT_EDITED","REFERENCE_SELECTED","VARIANT_SELECTED","VARIANT_REJECTED","AI_RECOMMENDATION_REJECTED","COMPOSITION_CHANGED","MATERIAL_SELECTED","TEXTURE_SELECTED","MANUAL_EDIT","APPROVED"} {
		if err := (Request{ActionType: action}).Validate(projectID, userID); err != nil {
			t.Fatalf("%s: unexpected error: %v", action, err)
		}
	}
}

func TestValidateRejectsUnknownAction(t *testing.T) {
	if err := (Request{ActionType: "INVENTED"}).Validate(uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected unknown action to be rejected")
	}
}
