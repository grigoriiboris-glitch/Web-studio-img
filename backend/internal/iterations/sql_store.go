package iterations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type SQLStore struct { db *sql.DB }

func NewSQLStore(db *sql.DB) (*SQLStore, error) {
	if db == nil { return nil, errors.New("iteration store requires database") }
	return &SQLStore{db: db}, nil
}

func (s *SQLStore) Create(ctx context.Context, userID, projectID uuid.UUID, parentID *uuid.UUID, iterationType Type, title, description *string) (Iteration, error) {
	if err := ValidateType(iterationType); err != nil { return Iteration{}, err }
	if err := ValidateTitle(title); err != nil { return Iteration{}, err }
	if err := ValidateDescription(description); err != nil { return Iteration{}, err }
	if err := ValidateManualEditDescription(iterationType, description); err != nil { return Iteration{}, err }
	if parentID != nil {
		parent, err := s.getOwned(ctx, userID, *parentID)
		if err != nil { return Iteration{}, err }
		if parent.ProjectID != projectID { return Iteration{}, ErrIterationNotFound }
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id = $1 AND user_id = $2 AND status <> 'deleted')`, projectID, userID).Scan(&exists); err != nil {
		return Iteration{}, fmt.Errorf("check project ownership: %w", err)
	}
	if !exists { return Iteration{}, ErrIterationNotFound }
	return s.insert(ctx, projectID, parentID, iterationType, title, description)
}

func (s *SQLStore) Get(ctx context.Context, userID, iterationID uuid.UUID) (Iteration, error) {
	return s.getOwned(ctx, userID, iterationID)
}

func (s *SQLStore) List(ctx context.Context, userID, projectID uuid.UUID) ([]Iteration, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.project_id, i.parent_iteration_id, i.type, i.title, i.description, i.created_at
		FROM iterations i
		JOIN projects p ON p.id = i.project_id
		WHERE i.project_id = $1 AND p.user_id = $2 AND p.status <> 'deleted'
		ORDER BY i.created_at ASC, i.id ASC
	`, projectID, userID)
	if err != nil { return nil, fmt.Errorf("list iterations: %w", err) }
	defer rows.Close()
	items := make([]Iteration, 0)
	for rows.Next() {
		item, err := scanIteration(rows)
		if err != nil { return nil, fmt.Errorf("scan iteration: %w", err) }
		items = append(items, item)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("list iterations: %w", err) }
	return items, nil
}

func (s *SQLStore) Restore(ctx context.Context, userID, iterationID uuid.UUID) (Iteration, error) {
	source, err := s.getOwned(ctx, userID, iterationID)
	if err != nil { return Iteration{}, err }
	return s.insert(ctx, source.ProjectID, &source.ID, source.Type, source.Title, source.Description)
}

func (s *SQLStore) getOwned(ctx context.Context, userID, iterationID uuid.UUID) (Iteration, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT i.id, i.project_id, i.parent_iteration_id, i.type, i.title, i.description, i.created_at
		FROM iterations i
		JOIN projects p ON p.id = i.project_id
		WHERE i.id = $1 AND p.user_id = $2 AND p.status <> 'deleted'
	`, iterationID, userID)
	item, err := scanIteration(row)
	if errors.Is(err, sql.ErrNoRows) { return Iteration{}, ErrIterationNotFound }
	if err != nil { return Iteration{}, fmt.Errorf("get iteration: %w", err) }
	return item, nil
}

func (s *SQLStore) insert(ctx context.Context, projectID uuid.UUID, parentID *uuid.UUID, iterationType Type, title, description *string) (Iteration, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO iterations (project_id, parent_iteration_id, type, title, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, project_id, parent_iteration_id, type, title, description, created_at
	`, projectID, parentID, iterationType, title, description)
	item, err := scanIteration(row)
	if err != nil { return Iteration{}, fmt.Errorf("create iteration: %w", err) }
	return item, nil
}

type scanner interface { Scan(...any) error }

func scanIteration(s scanner) (Iteration, error) {
	var item Iteration
	if err := s.Scan(&item.ID, &item.ProjectID, &item.ParentIterationID, &item.Type, &item.Title, &item.Description, &item.CreatedAt); err != nil {
		return Iteration{}, err
	}
	return item, nil
}
