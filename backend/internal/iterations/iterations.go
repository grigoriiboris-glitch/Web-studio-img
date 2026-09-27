package iterations

import (\n\t"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidIteration = errors.New("invalid iteration")
	ErrIterationNotFound = errors.New("iteration not found")
)

type Type string

const (
	TypeIdea Type = "idea"
	TypeSketch Type = "sketch"
	TypeGeneration Type = "generation"
	TypeSelection Type = "selection"
	TypeComposition Type = "composition"
	TypePrompt Type = "prompt"
	TypeManualEdit Type = "manual_edit"
	TypeFinal Type = "final"
)

var validTypes = map[Type]struct{}{
	TypeIdea: {}, TypeSketch: {}, TypeGeneration: {}, TypeSelection: {},
	TypeComposition: {}, TypePrompt: {}, TypeManualEdit: {}, TypeFinal: {},
}

type Iteration struct {
	ID uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	ParentIterationID *uuid.UUID `json:"parent_iteration_id,omitempty"`
	Type Type `json:"type"`
	Title *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Store interface {
	Create(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, Type, *string, *string) (Iteration, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Iteration, error)
	List(context.Context, uuid.UUID, uuid.UUID) ([]Iteration, error)
	Restore(context.Context, uuid.UUID, uuid.UUID) (Iteration, error)
}

func ValidateType(value Type) error {
	if _, ok := validTypes[value]; !ok { return ErrInvalidIteration }
	return nil
}

func ValidateTitle(title *string) error {
	if title == nil { return nil }
	if strings.TrimSpace(*title) == "" || len([]rune(*title)) > 200 { return ErrInvalidIteration }
	return nil
}

func ValidateDescription(description *string) error {
	if description == nil { return nil }
	if len([]rune(*description)) > 5000 { return ErrInvalidIteration }
	return nil
}
