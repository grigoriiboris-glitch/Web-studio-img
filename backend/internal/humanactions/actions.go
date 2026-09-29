package humanactions

import (
 "errors"
 "strings"
 "time"
 "github.com/google/uuid"
)

type Action struct {
 ID uuid.UUID `json:"id"`
 ProjectID uuid.UUID `json:"project_id"`
 IterationID *uuid.UUID `json:"iteration_id,omitempty"`
 UserID uuid.UUID `json:"user_id"`
 Version int64 `json:"version"`
 ActionType string `json:"action_type"`
 Payload map[string]any `json:"payload,omitempty"`
 OldState map[string]any `json:"old_state,omitempty"`
 NewState map[string]any `json:"new_state,omitempty"`
 AIInfluence map[string]any `json:"ai_influence,omitempty"`
 CreatedAt time.Time `json:"created_at"`
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
 "IDEA_CREATED": true, "SKETCH_IMPORTED": true, "PROMPT_EDITED": true, "PROMPT_APPROVED": true,
 "REFERENCE_ADDED": true, "REFERENCE_SELECTED": true, "VARIANT_SELECTED": true, "VARIANT_REJECTED": true,
 "AI_RECOMMENDATION_REJECTED": true, "COMPOSITION_CHANGED": true,
 "MATERIAL_SELECTED": true, "TEXTURE_SELECTED": true, "MANUAL_EDIT": true,
 "MATERIAL_CREATED": true, "MATERIAL_UPDATED": true, "MATERIAL_DELETED": true,
 "TEXTURE_CREATED": true, "TEXTURE_UPDATED": true, "TEXTURE_DELETED": true,
 "STYLE_PROFILE_CREATED": true, "STYLE_PROFILE_UPDATED": true, "STYLE_PROFILE_APPLIED": true,
 "ASSET_DNA_CREATED": true, "VISUAL_LANGUAGE_UPDATED": true,
 "RIGHTS_UPDATED": true, "DO_NOT_USE_UPDATED": true,
 "LAYER_CREATED": true, "LAYER_UPDATED": true, "LAYER_DELETED": true,
 "MASK_CREATED": true, "MANUAL_EDIT_CREATED": true, "MANUAL_EDIT_APPLIED": true, "MANUAL_EDIT_REJECTED": true,
 "APPROVED": true, "EXPORT_CREATED": true, "CREATIVE_BRIEF_CREATED": true, "CREATIVE_BRIEF_APPROVED": true,
}

func (r Request) Validate(projectID, userID uuid.UUID) error {
 if projectID == uuid.Nil || userID == uuid.Nil || !allowedActions[strings.TrimSpace(r.ActionType)] { return ErrInvalidAction }
 return nil
}
