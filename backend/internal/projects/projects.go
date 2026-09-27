package projects

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidProject = errors.New("invalid project")
	ErrProjectNotFound = errors.New("project not found")
)

type Status string

const (
	StatusActive Status = "active"
	StatusArchived Status = "archived"
	StatusDeleted Status = "deleted"
)

type Project struct {
	ID uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	Name string `json:"name"`
	Description *string `json:"description,omitempty"`
	Status Status `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	Create(context.Context, uuid.UUID, string, *string) (Project, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Project, error)
	List(context.Context, uuid.UUID) ([]Project, error)
	Update(context.Context, uuid.UUID, uuid.UUID, string, *string, Status) (Project, error)
	Archive(context.Context, uuid.UUID, uuid.UUID) (Project, error)
}

func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 200 {
		return "", ErrInvalidProject
	}
	return name, nil
}

func ValidateStatus(status Status) error {
	switch status {
	case StatusActive, StatusArchived, StatusDeleted:
		return nil
	default:
		return ErrInvalidProject
	}
}
