
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID         int64          `json:"id"`
	ProjectID  uuid.UUID      `json:"project_id"`
	UserID     uuid.UUID      `json:"user_id"`
	EventType  string         `json:"event_type"`
	EntityType string         `json:"entity_type"`
	EntityID   uuid.UUID      `json:"entity_id"`
	Payload    map[string]any `json:"payload,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

func EncodePayload(payload map[string]any) ([]byte, error) {
	if payload == nil { payload = map[string]any{} }
	return json.Marshal(payload)
}
