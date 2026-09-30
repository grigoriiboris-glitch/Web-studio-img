package branches

import (
    "errors"
    "strings"
    "time"

    "github.com/google/uuid"
)

var (
    ErrInvalidBranch = errors.New("invalid branch")
    ErrBranchNotFound = errors.New("branch not found")
    ErrBranchArchived = errors.New("branch archived")
    ErrMergeConflict = errors.New("merge conflict")
)

type Status string
const (
    StatusActive Status = "active"
    StatusArchived Status = "archived"
)

const MaxNameLength = 200
var decisionDimensions = []string{"subject","composition","prompt","material","texture","references","lighting","selected_asset","manual_edits"}

type Branch struct {
    ID uuid.UUID `json:"id"`
    ProjectID uuid.UUID `json:"project_id"`
    Name string `json:"name"`
    ParentIterationID *uuid.UUID `json:"parent_iteration_id,omitempty"`
    Status Status `json:"status"`
    CreatedBy uuid.UUID `json:"created_by"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Decision struct {
    SourceBranchID uuid.UUID `json:"source_branch_id"`
    Value any `json:"value"`
}

type CompareItem struct {
    Dimension string `json:"dimension"`
    Source any `json:"source"`
    Target any `json:"target"`
    Different bool `json:"different"`
}

type MergeRequest struct {
    Sources []uuid.UUID `json:"source_branch_ids"`
    Decisions map[string]Decision `json:"decisions"`
}

func ValidateName(name string) error {
    if strings.TrimSpace(name) == "" || len([]rune(name)) > MaxNameLength { return ErrInvalidBranch }
    return nil
}

func ValidateStatus(status Status) error {
    if status != StatusActive && status != StatusArchived { return ErrInvalidBranch }
    return nil
}

func DecisionDimensions() []string {
    return append([]string(nil), decisionDimensions...)
}

func IsDecisionDimension(value string) bool {
    for _, item := range decisionDimensions { if item == value { return true } }
    return false
}
