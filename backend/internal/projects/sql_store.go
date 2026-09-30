package projects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) (*SQLStore, error) {
	if db == nil {
		return nil, errors.New("project store requires database")
	}
	return &SQLStore{db: db}, nil
}

func (s *SQLStore) Create(ctx context.Context, userID uuid.UUID, name string, description *string) (Project, error) {
	name, err := ValidateName(name)
	if err != nil {
		return Project{}, err
	}
	return s.queryOne(ctx, `
		INSERT INTO projects (user_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, description, status, privacy_mode, created_at, updated_at
	`, userID, name, description)
}

func (s *SQLStore) Get(ctx context.Context, userID, projectID uuid.UUID) (Project, error) {
	return s.queryOne(ctx, `
		SELECT id, user_id, name, description, status, privacy_mode, created_at, updated_at
		FROM projects
		WHERE id = $1 AND user_id = $2 AND status <> 'deleted'
	`, projectID, userID)
}

func (s *SQLStore) List(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, description, status, created_at, updated_at
		FROM projects
		WHERE user_id = $1 AND status <> 'deleted'
		ORDER BY updated_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.UserID, &project.Name, &project.Description, &project.Status, &project.PrivacyMode, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		items = append(items, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return items, nil
}

func (s *SQLStore) Update(ctx context.Context, userID, projectID uuid.UUID, name string, description *string, status Status, privacyMode PrivacyMode) (Project, error) {
	name, err := ValidateName(name)
	if err != nil {
		return Project{}, err
	}
	if err := ValidateStatus(status); err != nil {
		return Project{}, err
	}
	return s.queryOne(ctx, `
		UPDATE projects
		SET name = $1, description = $2, status = $3, updated_at = now()
		WHERE id = $4 AND user_id = $5 AND status <> 'deleted'
		RETURNING id, user_id, name, description, status, created_at, updated_at
	`, name, description, status, projectID, userID)
}

func (s *SQLStore) Archive(ctx context.Context, userID, projectID uuid.UUID) (Project, error) {
	return s.queryOne(ctx, `
		UPDATE projects
		SET status = 'archived', updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status <> 'deleted'
		RETURNING id, user_id, name, description, status, created_at, updated_at
	`, projectID, userID)
}

func (s *SQLStore) queryOne(ctx context.Context, query string, args ...any) (Project, error) {
	var project Project
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&project.ID, &project.UserID, &project.Name, &project.Description,
		&project.Status, &project.PrivacyMode, &project.CreatedAt, &project.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrProjectNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("project query: %w", err)
	}
	return project, nil
}
