
package prompts

import (
	"errors"
	"strings"
	"time"
	"github.com/google/uuid"
)

type CreatedBy string
const (
	CreatedByHuman CreatedBy = "human"
	CreatedByAI CreatedBy = "ai"
	CreatedByMixed CreatedBy = "mixed"
)

var ErrInvalidPrompt = errors.New("invalid prompt")
var ErrPromptNotFound = errors.New("prompt not found")

var componentNames = map[string]bool{
	"subject":true,"composition":true,"camera":true,"lighting":true,"material":true,
	"texture":true,"color":true,"atmosphere":true,"style":true,"depth":true,
	"detail":true,"constraints":true,"negative_constraints":true,
}

var componentOrder = []string{
	"subject","composition","camera","lighting","material","texture","color",
	"atmosphere","style","depth","detail","constraints","negative_constraints",
}

func BuildFinalText(components map[string]string) string {
	parts := make([]string, 0, len(componentOrder))
	for _, name := range componentOrder {
		value := strings.TrimSpace(components[name])
		if value == "" {
			continue
		}
		if name == "negative_constraints" {
			parts = append(parts, "negative constraints: "+value)
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ", ")
}

type Prompt struct {
	ID uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	IterationID *uuid.UUID `json:"iteration_id,omitempty"`
	ParentPromptID *uuid.UUID `json:"parent_prompt_id,omitempty"`
	Version int `json:"version"`
	OriginalText string `json:"original_text"`
	AISuggestions []string `json:"ai_suggestions,omitempty"`
	FinalText *string `json:"final_text,omitempty"`
	Components map[string]string `json:"components,omitempty"`
	CreatedBy CreatedBy `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type Request struct {
	IterationID *uuid.UUID
	ParentPromptID *uuid.UUID
	OriginalText string
	AISuggestions []string
	FinalText *string
	Components map[string]string
	CreatedBy CreatedBy
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.OriginalText) == "" || len([]rune(r.OriginalText)) > 20000 { return ErrInvalidPrompt }
	if len(r.AISuggestions) > 20 || len(r.Components) > len(componentNames) { return ErrInvalidPrompt }
	if r.CreatedBy == "" { r.CreatedBy = CreatedByHuman }
	if r.CreatedBy != CreatedByHuman && r.CreatedBy != CreatedByAI && r.CreatedBy != CreatedByMixed { return ErrInvalidPrompt }
	for k,v := range r.Components {
		if !componentNames[k] || len([]rune(v)) > 4000 { return ErrInvalidPrompt }
	}
	for _, v := range r.AISuggestions {
		if len([]rune(v)) > 4000 { return ErrInvalidPrompt }
	}
	if r.FinalText != nil && len([]rune(*r.FinalText)) > 20000 { return ErrInvalidPrompt }
	return nil
}
