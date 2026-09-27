package assistant

import (
	"strings"
	"testing"

	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
)

func TestPromptSimilarityRequiresPersistedPromptText(t *testing.T) {
	if _, ok := promptSimilarity("", "stone"); ok {
		t.Fatal("expected missing prompt text to be unavailable")
	}
	if got, ok := promptSimilarity("stone", "stone"); !ok || got != 1 {
		t.Fatalf("expected persisted identical prompts to compare as 1, got %v, ok=%v", got, ok)
	}
}

func TestPromptJaccardNormalizesTokenContent(t *testing.T) {
	if got:=promptJaccard("Glass, warm light and stone", "stone / glass / warm / light / and"); got != 1 {
		t.Fatalf("expected identical token sets, got %v", got)
	}
	if got:=promptJaccard("", ""); got != 1 {
		t.Fatalf("expected empty prompts to compare as identical, got %v", got)
	}
	if got:=promptJaccard("red", "blue"); got != 0 {
		t.Fatalf("expected disjoint prompts to have zero overlap, got %v", got)
	}
}

func TestBuildCriticObservationsIsStructured(t *testing.T) {
	result:=similarity.Result{
		Visual:0.9, Composition:0.7, Semantic:0.4, Style:0.6,
		PHashScore:0.91, HistogramScore:0.89, EmbeddingScore:0.4,
	}
	compA:=map[string]any{
		"aspect_ratio":1.0,
		"focal_points":[]any{map[string]any{"x":0.25,"y":0.25}},
		"horizon":0.25,
		"perspective":"portrait-aspect",
		"negative_space":map[string]any{"edge_density":0.10},
		"object_scale":map[string]any{"heuristic_box_area":0.20},
		"bounding_boxes":[]any{map[string]any{"x_min":0.0,"y_min":0.0,"x_max":0.5,"y_max":0.5}},
	}
	compB:=map[string]any{
		"aspect_ratio":1.6,
		"focal_points":[]any{map[string]any{"x":0.7,"y":0.7}},
		"horizon":0.7,
		"perspective":"landscape-aspect",
		"negative_space":map[string]any{"edge_density":0.30},
		"object_scale":map[string]any{"heuristic_box_area":0.45},
		"bounding_boxes":[]any{map[string]any{"x_min":0.5,"y_min":0.5,"x_max":1.0,"y_max":1.0}},
	}
	observations:=buildCriticObservations(result,compA,compB,"red glass","blue stone")
	if len(observations)<6 {
		t.Fatalf("expected structured observations, got %d",len(observations))
	}
	for _,item:=range observations {
		for _,key:=range []string{"dimension","observation","reason","confidence"} {
			if _,ok:=item[key];!ok {
				t.Fatalf("missing observation field %q in %#v",key,item)
			}
		}
	}
	if observations[len(observations)-1]["dimension"]==nil {
		t.Fatal("observation dimension must be populated")
	}
	foundFocal := false
	foundPerspective := false
	for _, item := range observations {
		observation := item["observation"].(string)
		if strings.Contains(observation, "focal point") {
			foundFocal = true
		}
		if strings.Contains(observation, "perspective hint") {
			foundPerspective = true
		}
	}
	if !foundFocal || !foundPerspective {
		t.Fatalf("expected focal and perspective observations, got %#v", observations)
	}
}

func TestBuildCriticObservationsReportsHeuristicUncertainty(t *testing.T) {
	result:=similarity.Result{Visual:0.2,Composition:0.4,Semantic:0.3,Style:0.2}
	compA:=map[string]any{}
	compB:=map[string]any{}
	observations:=buildCriticObservations(result,compA,compB,"","")
	foundSemanticProxy:=false
	for _,item:=range observations {
		if item["dimension"]=="semantic" {
			foundSemanticProxy=true
			if !strings.Contains(item["reason"].(string),"proxy") {
				t.Fatalf("semantic observation must disclose proxy nature: %#v",item)
			}
		}
		if confidence,ok:=item["confidence"].(float64); !ok || confidence < 0 || confidence > 1 {
			t.Fatalf("confidence out of range: %#v",item["confidence"])
		}
	}
	if !foundSemanticProxy { t.Fatal("semantic proxy observation missing") }
}

func TestFocalPointRejectsInvalidCoordinates(t *testing.T) {
	if got:=focalPoint(map[string]any{
		"focal_points":[]any{map[string]any{"x":1.4,"y":0.2}},
	}); got != nil {
		t.Fatalf("expected invalid focal point to be ignored, got %#v",got)
	}
}


func TestBuildCriticObservationsDoesNotInventMissingPromptSimilarity(t *testing.T) {
	observations := buildCriticObservations(
		similarity.Result{Visual: 0.5, Composition: 0.5, Semantic: 0.5, Style: 0.5},
		map[string]any{},
		map[string]any{},
		"",
		"",
	)
	for _, item := range observations {
		if item["dimension"] == "prompt" && item["observation"] == "Prompts are highly overlapping by token content." {
			t.Fatal("critic must not infer prompt similarity when prompt text is missing")
		}
	}
}
