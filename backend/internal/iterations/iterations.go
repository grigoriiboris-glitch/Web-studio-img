package iterations

import (
    "context"
    "encoding/json"
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
    TypeIdea Type = "idea"; TypeSketch Type = "sketch"; TypeGeneration Type = "generation"; TypeSelection Type = "selection"
    TypeComposition Type = "composition"; TypePrompt Type = "prompt"; TypeManualEdit Type = "manual_edit"; TypeFinal Type = "final"
)
var validTypes=map[Type]struct{}{TypeIdea:{},TypeSketch:{},TypeGeneration:{},TypeSelection:{},TypeComposition:{},TypePrompt:{},TypeManualEdit:{},TypeFinal:{}}
type Iteration struct {
    ID uuid.UUID `json:"id"`
    ProjectID uuid.UUID `json:"project_id"`
    BranchID uuid.UUID `json:"branch_id"`
    ParentIterationID *uuid.UUID `json:"parent_iteration_id,omitempty"`
    Type Type `json:"type"`
    Title *string `json:"title,omitempty"`
    Description *string `json:"description,omitempty"`
    Decisions map[string]any `json:"decisions,omitempty"`
    MergeSources []uuid.UUID `json:"merge_sources,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}
type Store interface {
    Create(context.Context,uuid.UUID,uuid.UUID,*uuid.UUID,Type,*string,*string)(Iteration,error)
    Get(context.Context,uuid.UUID,uuid.UUID)(Iteration,error)
    List(context.Context,uuid.UUID,uuid.UUID)([]Iteration,error)
    Restore(context.Context,uuid.UUID,uuid.UUID)(Iteration,error)
}
func ValidateType(v Type)error{if _,ok:=validTypes[v];!ok{return ErrInvalidIteration};return nil}
func ValidateTitle(v *string)error{if v==nil{return nil};if strings.TrimSpace(*v)==""||len([]rune(*v))>200{return ErrInvalidIteration};return nil}
func ValidateDescription(v *string)error{if v==nil{return nil};if len([]rune(*v))>5000{return ErrInvalidIteration};return nil}
func ValidateManualEditDescription(t Type,v *string)error{if t!=TypeManualEdit{return nil};if v==nil||strings.TrimSpace(*v)==""{return ErrInvalidIteration};return ValidateDescription(v)}
func ValidateDecisions(v map[string]any)error{if v==nil{return nil};for k,value:=range v{switch k{case "subject","composition","prompt","material","texture","references","lighting","selected_asset","manual_edits":continue;case "reject_constraints":var constraints []struct{Reason string `json:"reason"`;Count int `json:"count"`;Percent float64 `json:"percent"`};raw,err:=json.Marshal(value);if err!=nil||json.Unmarshal(raw,&constraints)!=nil||len(constraints)>5{return ErrInvalidIteration};seen:=map[string]struct{}{};for _,constraint:=range constraints{if !rejectReasonAllowed(constraint.Reason)||constraint.Count<0||constraint.Percent<0{return ErrInvalidIteration};if _,exists:=seen[constraint.Reason];exists{return ErrInvalidIteration};seen[constraint.Reason]=struct{}{}};default:return ErrInvalidIteration}};_,err:=json.Marshal(v);return err}
func rejectReasonAllowed(reason string)bool{switch strings.ToLower(strings.TrimSpace(reason)){case "composition","subject","pose","lighting","color","material","background","object","style","prompt","quality","other":return true;default:return false}}
