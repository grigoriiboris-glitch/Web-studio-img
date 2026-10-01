package iterations

import "testing"

func TestValidateDecisionsRejectConstraints(t *testing.T) {
  valid := map[string]any{
    "reject_constraints": []any{
      map[string]any{"reason": "composition", "count": 4, "percent": 40},
      map[string]any{"reason": "lighting", "count": 2, "percent": 20},
    },
  }
  if err := ValidateDecisions(valid); err != nil {
    t.Fatalf("valid reject constraints rejected: %v", err)
  }
}

func TestValidateDecisionsRejectConstraintsRejectsInvalid(t *testing.T) {
  tests := []map[string]any{
    {"reject_constraints": []any{map[string]any{"reason": "camera", "count": 1, "percent": 10}}},
    {"reject_constraints": []any{map[string]any{"reason": "composition", "count": -1, "percent": 10}}},
    {"reject_constraints": []any{
      map[string]any{"reason": "composition", "count": 1, "percent": 10},
      map[string]any{"reason": "composition", "count": 2, "percent": 20},
    }},
  }
  for i, decisions := range tests {
    if err := ValidateDecisions(decisions); err == nil {
      t.Fatalf("case %d: expected validation error", i)
    }
  }
}
