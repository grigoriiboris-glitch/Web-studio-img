
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil { return nil, errors.New("events store requires database") }
	return &Store{db: db}, nil
}

func (s *Store) ProjectOwned(ctx context.Context, userID, projectID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')", projectID, userID).Scan(&exists)
	return exists, err
}

func (s *Store) Append(ctx context.Context, userID, projectID uuid.UUID, eventType, entityType string, entityID uuid.UUID, payload map[string]any) (Event, error) {
	raw, err := EncodePayload(payload)
	if err != nil { return Event{}, fmt.Errorf("marshal project event: %w", err) }
	var event Event
	var stored []byte
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO project_events(user_id, project_id, event_type, entity_type, entity_id, payload)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING sequence, project_id, user_id, event_type, entity_type, entity_id, payload, created_at
	`, userID, projectID, eventType, entityType, entityID, raw).Scan(
		&event.ID, &event.ProjectID, &event.UserID, &event.EventType, &event.EntityType, &event.EntityID, &stored, &event.CreatedAt,
	)
	if err != nil { return Event{}, fmt.Errorf("append project event: %w", err) }
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &event.Payload); err != nil { return Event{}, fmt.Errorf("decode project event: %w", err) }
	}
	return event, nil
}

func (s *Store) ListSince(ctx context.Context, userID, projectID uuid.UUID, after int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 { limit = 100 }
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.sequence, e.project_id, e.user_id, e.event_type, e.entity_type, e.entity_id, e.payload, e.created_at
		FROM project_events e JOIN projects p ON p.id=e.project_id
		WHERE e.project_id=$1 AND e.user_id=$2 AND p.user_id=$2 AND p.status <> 'deleted' AND e.sequence > $3
		ORDER BY e.sequence ASC LIMIT $4
	`, projectID, userID, after, limit)
	if err != nil { return nil, fmt.Errorf("list project events: %w", err) }
	defer rows.Close()
	var result []Event
	for rows.Next() {
		var item Event
		var raw []byte
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.UserID, &item.EventType, &item.EntityType, &item.EntityID, &raw, &item.CreatedAt); err != nil { return nil, err }
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &item.Payload); err != nil { return nil, err }
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
