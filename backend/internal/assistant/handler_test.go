package assistant

import "testing"

func TestToolCatalogIsComplete(t *testing.T) {
	required := map[string]bool{
		"create_iteration": true,
		"get_project_history": true,
		"analyze_composition": true,
		"search_references": true,
		"check_similarity": true,
		"suggest_prompt": true,
		"suggest_materials": true,
		"create_generation": true,
		"compare_iterations": true,
		"verify_provenance": true,
		"fact_check": true,
	}
	for _, tool := range toolCatalog { delete(required, tool.Name) }
	if len(required) != 0 { t.Fatalf("missing tools: %#v", required) }
}

func TestRecommendationContract(t *testing.T) {
	result := withRecommendation("suggest_prompt", map[string]any{
		"suggested_prompt": "A clearer focal point",
		"iteration_id": "iteration-1",
	}, "Derived from stored prompt context.", 0.63, "Heuristic MVP")
	r, ok := result["recommendation"].(map[string]any)
	if !ok { t.Fatal("recommendation contract missing") }
	for _, key := range []string{"recommendation", "reason", "evidence", "confidence", "affected_entity", "expected_effect"} {
		if _, ok := r[key]; !ok { t.Fatalf("recommendation field %q missing", key) }
	}
	if r["confidence"] != 0.63 { t.Fatalf("unexpected confidence: %#v", r["confidence"]) }
	if r["evidence"].(map[string]any)["suggested_prompt"] != "A clearer focal point" { t.Fatal("evidence must preserve original result") }
}


func TestRecommendationSemanticsAreExplicit(t *testing.T) {
	composition := withRecommendation("analyze_composition", map[string]any{
		"asset_id": "asset-1",
		"iteration_id": "iteration-1",
	}, "Heuristic composition descriptors.", 0.62, "No learned vision model is used.")
	recommendation, ok := composition["recommendation"].(map[string]any)
	if !ok {
		t.Fatal("recommendation missing")
	}
	if recommendation["affected_entity"].(map[string]any)["type"] != "iteration" {
		t.Fatalf("expected iteration as affected entity, got %#v", recommendation["affected_entity"])
	}
	if recommendation["expected_effect"] == "" {
		t.Fatal("expected effect must be explicit")
	}
}


func TestMaterialRecommendationApplyIsDecisionOnly(t *testing.T) {
	result := withRecommendation("suggest_materials", map[string]any{
		"kind": "material",
		"items": []any{map[string]any{"name": "Stone"}},
	}, "Project-visible material suggestions.", 0.90, "Ranking is search/recency based.")
	recommendation := result["recommendation"].(map[string]any)
	if recommendation["expected_effect"] != "Accept the suggested project-visible materials or textures for later manual selection; no project mutation occurs automatically." {
		t.Fatalf("unexpected material expected effect: %#v", recommendation["expected_effect"])
	}
}


func TestNormalizeFlowStepsPreservesArtistOrderAndEnabledState(t *testing.T) {
	steps := normalizeFlowSteps([]any{
		map[string]any{"id":"reference","enabled":false},
		map[string]any{"id":"sketch","enabled":true},
		map[string]any{"id":"structure","enabled":true},
		map[string]any{"id":"final","enabled":true},
	})
	if len(steps) != 4 { t.Fatalf("expected 4 steps, got %d", len(steps)) }
	if steps[0]["id"] != "reference" || steps[0]["enabled"] != false {
		t.Fatalf("unexpected first step: %#v", steps[0])
	}
	if steps[1]["id"] != "sketch" || steps[1]["order"] != 1 {
		t.Fatalf("unexpected second step: %#v", steps[1])
	}
}

func TestNormalizeFlowStepsRejectsUnknownAndDuplicateSteps(t *testing.T) {
	steps := normalizeFlowSteps([]any{
		map[string]any{"id":"unknown","enabled":true},
		map[string]any{"id":"sketch","enabled":true},
		map[string]any{"id":"sketch","enabled":false},
	})
	if len(steps) != 1 || steps[0]["id"] != "sketch" || steps[0]["enabled"] != true {
		t.Fatalf("unexpected normalized steps: %#v", steps)
	}
}

func TestFormatFlowStepsIsDeterministic(t *testing.T) {
	got := formatFlowSteps([]map[string]any{
		{"id":"sketch","enabled":true,"order":0},
		{"id":"reference","enabled":false,"order":1},
	})
	want := "1. sketch (enabled)\n2. reference (disabled)"
	if got != want { t.Fatalf("got %q, want %q", got, want) }
}


func TestValidateArtistFlowContractAcceptsMatchingEvidence(t *testing.T) {
	expected := []map[string]any{
		{"id":"sketch","enabled":true,"order":0},
		{"id":"reference","enabled":true,"order":1},
		{"id":"structure","enabled":true,"order":2},
		{"id":"final","enabled":true,"order":3},
	}
	actual := []map[string]any{
		{"id":"sketch","enabled":true,"order":0,"evidence":[]any{map[string]any{"kind":"asset","value":"sketch"}}},
		{"id":"reference","enabled":true,"order":1,"evidence":[]any{map[string]any{"kind":"asset","value":"color_reference"}}},
		{"id":"structure","enabled":true,"order":2,"evidence":[]any{map[string]any{"kind":"node","value":"2"}}},
		{"id":"final","enabled":true,"order":3,"evidence":[]any{map[string]any{"kind":"node","value":"3"}}},
	}
	workflow := map[string]any{
		"1": map[string]any{"inputs": map[string]any{"image":"{{asset:sketch}}"}},
		"2": map[string]any{"class_type":"ControlNetApply"},
		"3": map[string]any{"class_type":"SaveImage"},
		"4": map[string]any{"inputs": map[string]any{"image":"{{asset:color_reference}}"}},
	}
	if errors := validateArtistFlowContract(expected, actual, workflow); len(errors) != 0 {
		t.Fatalf("unexpected semantic errors: %#v", errors)
	}
}

func TestValidateArtistFlowContractRejectsChangedOrderAndDisabledState(t *testing.T) {
	expected := []map[string]any{
		{"id":"sketch","enabled":true,"order":0},
		{"id":"reference","enabled":false,"order":1},
		{"id":"final","enabled":true,"order":2},
	}
	actual := []map[string]any{
		{"id":"reference","enabled":true,"order":0,"evidence":[]any{map[string]any{"kind":"asset","value":"color_reference"}}},
		{"id":"sketch","enabled":true,"order":1,"evidence":[]any{map[string]any{"kind":"asset","value":"sketch"}}},
		{"id":"final","enabled":true,"order":2,"evidence":[]any{map[string]any{"kind":"node","value":"3"}}},
	}
	workflow := map[string]any{
		"1": map[string]any{"inputs": map[string]any{"image":"{{asset:sketch}}"}},
		"3": map[string]any{"class_type":"SaveImage"},
		"4": map[string]any{"inputs": map[string]any{"image":"{{asset:color_reference}}"}},
	}
	errors := validateArtistFlowContract(expected, actual, workflow)
	if len(errors) == 0 {
		t.Fatal("expected semantic validation errors")
	}
}

func TestValidateArtistFlowContractRejectsMissingEvidence(t *testing.T) {
	expected := []map[string]any{
		{"id":"sketch","enabled":true,"order":0},
	}
	actual := []map[string]any{
		{"id":"sketch","enabled":true,"order":0,"evidence":[]any{}},
	}
	errors := validateArtistFlowContract(expected, actual, map[string]any{})
	if len(errors) == 0 {
		t.Fatal("expected missing evidence error")
	}
}

func TestFlowFingerprintIsStableAndSensitiveToFlowState(t *testing.T) {
	a := []map[string]any{
		{"id":"sketch","enabled":true,"order":0},
		{"id":"reference","enabled":true,"order":1},
	}
	b := []map[string]any{
		{"id":"sketch","enabled":true,"order":0},
		{"id":"reference","enabled":true,"order":1},
	}
	c := []map[string]any{
		{"id":"sketch","enabled":true,"order":0},
		{"id":"reference","enabled":false,"order":1},
	}
	if flowFingerprint(a) != flowFingerprint(b) {
		t.Fatal("equivalent flows must have the same fingerprint")
	}
	if flowFingerprint(a) == flowFingerprint(c) {
		t.Fatal("changing enabled state must change the fingerprint")
	}
}


func TestColorReferenceContractRequiresReferenceInputAndWorkflowEvidence(t *testing.T) {
	steps := []map[string]any{{"id":"sketch","enabled":true,"order":0},{"id":"reference","enabled":true,"order":1},{"id":"final","enabled":true,"order":2}}
	inputs := []FlowInput{{ID:"sketch",Type:"image",Label:"Sketch",Required:true}}
	workflow := map[string]any{"1":map[string]any{"inputs":map[string]any{"image":"{{asset:sketch}}"}}}
	errors := colorReferenceContractErrors(steps, inputs, workflow)
	if len(errors) != 2 { t.Fatalf("expected two color-reference contract errors, got %#v", errors) }
}

func TestColorReferenceContractAcceptsRequiredInputAndEvidence(t *testing.T) {
	steps := []map[string]any{{"id":"sketch","enabled":true,"order":0},{"id":"reference","enabled":true,"order":1},{"id":"final","enabled":true,"order":2}}
	inputs := []FlowInput{
		{ID:"sketch",Type:"image",Label:"Sketch",Required:true},
		{ID:"color_reference",Type:"image",Label:"Color Reference",Required:true},
	}
	workflow := map[string]any{
		"1":map[string]any{"inputs":map[string]any{"image":"{{asset:sketch}}"}},
		"2":map[string]any{"inputs":map[string]any{"image":"{{asset:color_reference}}"}},
	}
	if errors := colorReferenceContractErrors(steps, inputs, workflow); len(errors) != 0 {
		t.Fatalf("unexpected color-reference contract errors: %#v", errors)
	}
}

func TestColorReferenceContractDoesNotRequireReferenceWhenDisabled(t *testing.T) {
	steps := []map[string]any{{"id":"sketch","enabled":true,"order":0},{"id":"reference","enabled":false,"order":1},{"id":"final","enabled":true,"order":2}}
	inputs := []FlowInput{{ID:"sketch",Type:"image",Label:"Sketch",Required:true}}
	workflow := map[string]any{"1":map[string]any{"inputs":map[string]any{"image":"{{asset:sketch}}"}}}
	if errors := colorReferenceContractErrors(steps, inputs, workflow); len(errors) != 0 {
		t.Fatalf("disabled reference step must not require color reference: %#v", errors)
	}
}
