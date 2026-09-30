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
