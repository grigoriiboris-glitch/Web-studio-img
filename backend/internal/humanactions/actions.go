package humanactions

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Action struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"project_id"`
	IterationID *uuid.UUID `json:"iteration_id,omitempty"`
	UserID      uuid.UUID `json:"user_id"`
	ActionType  string `json:"action_type"`
	Payload     map[string]any `json:"payload,omitempty"`
	OldState    map[string]any `json:"old_state,omitempty"`
	NewState    map[string]any `json:"new_state,omitempty"`
	AIInfluence map[string]any `json:"ai_influence,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Request struct {
	IterationID *uuid.UUID `json:"iteration_id,omitempty"`
	ActionType string `json:"action_type"`
	Payload map[string]any `json:"payload,omitempty"`
	OldState map[string]any `json:"old_state,omitempty"`
	NewState map[string]any `json:"new_state,omitempty"`
	AIInfluence map[string]any `json:"ai_influence,omitempty"`
}

var ErrInvalidAction = errors.New("invalid human action")

var allowedActions = map[string]bool{
	"IDEA_CREATED": true, "PROMPT_EDITED": true, "PROMPT_APPROVED": true,
	"REFERENCE_ADDED": true, "REFERENCE_SELECTED": true, "VARIANT_SELECTED": true, "VARIANT_REJECTED": true,
	"AI_RECOMMENDATION_REJECTED": true,
	"COMPOSITION_CHANGED": true, "MATERIAL_SELECTED": true, "TEXTURE_SELECTED": true,
	"MANUAL_EDIT": true, "APPROVED": true, "EXPORT_CREATED": true,
}

func (r Request) Validate(projectID, userID uuid.UUID) error {
	if projectID == uuid.Nil || userID == uuid.Nil || !allowedActions[strings.TrimSpace(r.ActionType)] {
		return ErrInvalidAction
	}
	return nil
}
