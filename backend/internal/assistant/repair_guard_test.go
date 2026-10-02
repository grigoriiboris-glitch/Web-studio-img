package assistant

import "testing"

func TestCheckRepairInvariantsRejectsProtectedChanges(t *testing.T) {
    input := RepairInvariantInput{
        FlowSteps: []map[string]any{
            {"id":"sketch","enabled":true,"order":0},
            {"id":"reference","enabled":true,"order":1},
        },
        Inputs: map[string]any{"sketch": "asset-1", "color_reference": "asset-2"},
        CreativeBrief: map[string]any{"goal":"preserve character"},
        ColorReferenceContract: map[string]any{"mode":"palette"},
        ArtistIntent: "keep linework",
    }
    before := map[string]any{
        "1": map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:sketch}}"}},
        "2": map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:color_reference}}"}},
    }
    after := map[string]any{
        "1": map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:sketch}}"}},
        "2": map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:other}}"}},
    }
    candidate := map[string]any{
        "flow_steps": []any{map[string]any{"id":"sketch","enabled":true,"order":0}, map[string]any{"id":"reference","enabled":false,"order":1}},
        "inputs": map[string]any{"sketch":"asset-1","color_reference":"asset-3"},
        "creative_brief": map[string]any{"goal":"redesign"},
        "color_reference_contract": map[string]any{"mode":"none"},
        "artist_intent": "change linework",
    }
    for key, value := range candidate { after[key] = value }

    got := checkRepairInvariants(input, before, after)
    if got.Valid { t.Fatalf("expected invariant violation, got %+v", got) }
    if len(got.Violations) < 5 { t.Fatalf("expected protected-field and asset violations, got %+v", got) }
    if got.WorkflowBefore == got.WorkflowAfter { t.Fatal("workflow fingerprint did not change") }
}

func TestCheckRepairInvariantsAllowsTechnicalWorkflowRepair(t *testing.T) {
    input := RepairInvariantInput{
        FlowSteps: []map[string]any{{"id":"sketch","enabled":true,"order":0}},
        Inputs: map[string]any{"sketch":"asset-1"},
        CreativeBrief: map[string]any{"goal":"preserve"},
        ColorReferenceContract: map[string]any{"mode":"palette"},
        ArtistIntent: "preserve intent",
    }
    before := map[string]any{"1":map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:sketch}}"}}}
    after := map[string]any{"1":map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:sketch}}"}}, "2":map[string]any{"class_type":"VAEDecode","inputs":map[string]any{}}}
    got := checkRepairInvariants(input, before, after)
    if !got.Valid { t.Fatalf("technical repair rejected: %+v", got) }
    if got.WorkflowBefore == got.WorkflowAfter { t.Fatal("expected different workflow fingerprint") }
}

func TestCheckRepairInvariantsProtectedFingerprintIsStable(t *testing.T) {
    input := RepairInvariantInput{
        FlowSteps: []map[string]any{{"id":"sketch","enabled":true,"order":0}},
        Inputs: map[string]any{"sketch":"asset-1"},
        CreativeBrief: "brief",
        ColorReferenceContract: "palette",
        ArtistIntent: "intent",
    }
    if got := hashProtectedRepairState(input); len(got) != 64 { t.Fatalf("protected fingerprint length=%d", len(got)) }
}
