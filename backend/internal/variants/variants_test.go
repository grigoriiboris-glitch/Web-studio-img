package variants

import "testing"

func TestNormalizeRejectReasons(t *testing.T) {
  tests := []struct {
    name    string
    input   []string
    want    []string
    wantErr bool
  }{
    {name: "normalizes and sorts", input: []string{" Lighting ", "composition"}, want: []string{"composition", "lighting"}},
    {name: "allows all MVP reasons", input: []string{"composition","subject","pose","lighting","color","material","background","object","style","prompt","quality","other"}, want: []string{"background","color","composition","lighting","material","object","other","pose","prompt","quality","style","subject"}},
    {name: "rejects unknown", input: []string{"camera"}, wantErr: true},
    {name: "rejects duplicate", input: []string{"color","Color"}, wantErr: true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      got, err := normalizeRejectReasons(tt.input)
      if tt.wantErr {
        if err == nil {
          t.Fatal("expected error")
        }
        return
      }
      if err != nil {
        t.Fatalf("unexpected error: %v", err)
      }
      if len(got) != len(tt.want) {
        t.Fatalf("got %v, want %v", got, tt.want)
      }
      for i := range got {
        if got[i] != tt.want[i] {
          t.Fatalf("got %v, want %v", got, tt.want)
        }
      }
    })
  }
}

func TestVariantDecisionStateIncludesRejectMetadata(t *testing.T) {
  comment := "too centered"
  severity := "high"
  state := variantDecisionState(Variant{
    Decision:            "rejected",
    RejectReason:        []string{"composition", "lighting"},
    RejectComment:       &comment,
    RejectSeverity:      &severity,
    RejectReasonSkipped: false,
  })

  if state["decision"] != "rejected" {
    t.Fatalf("unexpected decision: %#v", state["decision"])
  }
  reasons := state["reject_reason"].([]string)
  if len(reasons) != 2 || reasons[0] != "composition" || reasons[1] != "lighting" {
    t.Fatalf("unexpected reasons: %#v", reasons)
  }
  if state["reject_comment"] != &comment {
    t.Fatalf("unexpected comment: %#v", state["reject_comment"])
  }
  if state["reject_severity"] != &severity {
    t.Fatalf("unexpected severity: %#v", state["reject_severity"])
  }
}


func TestValidateSelectionResult(t *testing.T) {
  tests := []struct {
    name string
    selected, requested, branches int
    parentValid, branchValid bool
    wantErr bool
  }{
    {name: "single branch selection", selected: 2, requested: 2, branches: 1, parentValid: true, branchValid: true},
    {name: "rejects non-kept variant", selected: 1, requested: 2, branches: 1, parentValid: true, branchValid: true, wantErr: true},
    {name: "rejects mixed branches", selected: 2, requested: 2, branches: 2, parentValid: true, branchValid: true, wantErr: true},
    {name: "rejects missing parent", selected: 2, requested: 2, branches: 1, parentValid: false, branchValid: true, wantErr: true},
    {name: "rejects missing branch", selected: 2, requested: 2, branches: 1, parentValid: true, branchValid: false, wantErr: true},
  }
  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      err := validateSelectionResult(tt.selected, tt.requested, tt.branches, tt.parentValid, tt.branchValid)
      if (err != nil) != tt.wantErr {
        t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
      }
    })
  }
}

func TestMutationRequestHashIsDeterministic(t *testing.T) {
  payload := map[string]any{
    "project_id": "project-1",
    "generation_ids": []string{"g1", "g2"},
    "name": "Candidates",
  }
  first := mutationRequestHash("create_set", payload)
  second := mutationRequestHash("create_set", payload)
  if first != second {
    t.Fatalf("hash changed between identical requests: %s != %s", first, second)
  }
  if len(first) != 64 {
    t.Fatalf("expected SHA-256 hex hash, got %q", first)
  }
  if first == mutationRequestHash("create_set", map[string]any{"project_id": "project-1", "generation_ids": []string{"g1", "g3"}, "name": "Candidates"}) {
    t.Fatal("different request payload produced the same hash")
  }
}

func TestMutationRequestHashIncludesOperation(t *testing.T) {
  payload := map[string]any{"id": "same"}
  if mutationRequestHash("patch_variant", payload) == mutationRequestHash("create_set", payload) {
    t.Fatal("different mutation operations must produce different hashes")
  }
}

func TestValidateDecisionTransition(t *testing.T) {
  allowed := [][2]string{
    {"candidate", "kept"},
    {"candidate", "selected"},
    {"candidate", "rejected"},
    {"kept", "selected"},
    {"kept", "rejected"},
    {"selected", "rejected"},
    {"rejected", "candidate"},
    {"candidate", "candidate"},
    {"kept", "kept"},
    {"selected", "selected"},
    {"rejected", "rejected"},
  }
  for _, pair := range allowed {
    if err := validateDecisionTransition(pair[0], pair[1]); err != nil {
      t.Fatalf("expected transition %s -> %s to be allowed: %v", pair[0], pair[1], err)
    }
  }

  rejected := [][2]string{
    {"kept", "candidate"},
    {"selected", "candidate"},
    {"selected", "kept"},
    {"rejected", "kept"},
    {"rejected", "selected"},
  }
  for _, pair := range rejected {
    if err := validateDecisionTransition(pair[0], pair[1]); err == nil {
      t.Fatalf("expected transition %s -> %s to be rejected", pair[0], pair[1])
    }
  }
}
