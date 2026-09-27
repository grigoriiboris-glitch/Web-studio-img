package assistant

import (
	"testing"

	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
)

func TestPromptJaccardNormalizesTokenContent(t *testing.T) {
	if got:=promptJaccard("Glass, warm light and stone", "stone / glass / warm light"); got != 1 {
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
	}
	compB:=map[string]any{
		"aspect_ratio":1.6,
		"focal_points":[]any{map[string]any{"x":0.7,"y":0.7}},
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
}
