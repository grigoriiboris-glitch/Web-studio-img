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
