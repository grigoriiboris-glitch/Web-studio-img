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
