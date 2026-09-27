package assistant

import (
	"sort"
	"strings"
	"unicode"

	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
)

type criticAsset struct {
	ID        string
	StorageKey string
	Prompt    string
	HasAsset  bool
}

func criticObservation(dimension, observation, reason string, confidence float64) map[string]any {
	return map[string]any{
		"dimension":   dimension,
		"observation": observation,
		"reason":      reason,
		"confidence": confidence,
	}
}

func buildCriticObservations(result similarity.Result, compA, compB map[string]any, promptA, promptB string) []map[string]any {
	observations := make([]map[string]any, 0, 6)

	addScoreObservation := func(dimension, label, reason string, a, b float64) {
		delta := b - a
		switch {
		case delta > 0.15:
			observations = append(observations, criticObservation(
				dimension,
				label+" is materially higher in iteration B.",
				reason,
				0.86,
			))
		case delta < -0.15:
			observations = append(observations, criticObservation(
				dimension,
				label+" is materially lower in iteration B.",
				reason,
				0.86,
			))
		default:
			observations = append(observations, criticObservation(
				dimension,
				label+" is broadly similar between the iterations.",
				reason,
				0.78,
			))
		}
	}

	addScoreObservation("visual", "Visual similarity", "Perceptual hash and luminance histogram comparison.", result.Visual, result.Visual)
	addScoreObservation("composition", "Composition similarity", "Heuristic focal-center, edge-density, and aspect-ratio comparison.", result.Composition, result.Composition)
	addScoreObservation("semantic", "Semantic similarity proxy", "Image histogram/color embedding cosine similarity; this is a proxy, not a semantic model.", result.Semantic, result.Semantic)
	addScoreObservation("style", "Style similarity", "Heuristic comparison of edge density, luminance variance, and histogram.", result.Style, result.Style)

	aspectA, aspectB := numberFrom(compA["aspect_ratio"]), numberFrom(compB["aspect_ratio"])
	if aspectA > 0 && aspectB > 0 {
		delta := absFloat(aspectB - aspectA)
		observations = append(observations, criticObservation(
			"composition",
			formatCompositionAspectObservation(delta),
			"Aspect ratios are compared from decoded image dimensions.",
			0.98,
		))
	}

	focalA := focalPoint(compA)
	focalB := focalPoint(compB)
	if focalA != nil && focalB != nil {
		distance := hypot(focalA[0]-focalB[0], focalA[1]-focalB[1])
		observations = append(observations, criticObservation(
			"composition",
			formatFocalObservation(distance),
			"Focal-point coordinates are estimated from luminance-weighted image descriptors.",
			0.72,
		))
	}

	promptScore := promptJaccard(promptA, promptB)
	observations = append(observations, criticObservation(
		"prompt",
		formatPromptObservation(promptScore),
		"Prompt similarity is a token-set Jaccard comparison of persisted prompt text.",
		0.95,
	))

	sort.SliceStable(observations, func(i, j int) bool {
		return observations[i]["dimension"].(string) < observations[j]["dimension"].(string)
	})
	return observations
}

func promptJaccard(a, b string) float64 {
	aTokens := promptTokens(a)
	bTokens := promptTokens(b)
	if len(aTokens) == 0 && len(bTokens) == 0 {
		return 1
	}
	union := make(map[string]struct{}, len(aTokens)+len(bTokens))
	intersection := 0
	for token := range aTokens {
		union[token] = struct{}{}
	}
	for token := range bTokens {
		if _, ok := aTokens[token]; ok {
			intersection++
		}
		union[token] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func promptTokens(value string) map[string]struct{} {
	tokens := make(map[string]struct{})
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens[strings.ToLower(current.String())] = struct{}{}
		current.Reset()
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func focalPoint(value map[string]any) []float64 {
	raw, ok := value["focal_points"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	point, ok := raw[0].(map[string]any)
	if !ok {
		return nil
	}
	x, y := numberFrom(point["x"]), numberFrom(point["y"])
	if x < 0 || x > 1 || y < 0 || y > 1 {
		return nil
	}
	return []float64{x, y}
}

func numberFrom(value any) float64 {
	switch n := value.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func formatCompositionAspectObservation(delta float64) string {
	switch {
	case delta < 0.05:
		return "Aspect ratio is nearly unchanged."
	case delta < 0.2:
		return "Aspect ratio changed moderately."
	default:
		return "Aspect ratio changed substantially."
	}
}

func formatFocalObservation(distance float64) string {
	switch {
	case distance < 0.05:
		return "The estimated focal point is nearly unchanged."
	case distance < 0.2:
		return "The estimated focal point moved moderately."
	default:
		return "The estimated focal point moved substantially."
	}
}

func formatPromptObservation(score float64) string {
	switch {
	case score >= 0.8:
		return "Prompts are highly overlapping by token content."
	case score >= 0.5:
		return "Prompts partially overlap by token content."
	case score > 0:
		return "Prompts have limited token overlap."
	default:
		return "Prompts share no normalized tokens."
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func hypot(x, y float64) float64 {
	if x < 0 {
		x = -x
	}
	if y < 0 {
		y = -y
	}
	switch {
	case x == 0:
		return y
	case y == 0:
		return x
	case x > y:
		r := y / x
		return x * sqrt1p(r*r)
	default:
		r := x / y
		return y * sqrt1p(r*r)
	}
}

func sqrt1p(v float64) float64 {
	// Newton's method is deterministic and avoids introducing another math dependency surface.
	x := 1.0
	if v > 0 {
		x = 1 + v/2
	}
	for i := 0; i < 8; i++ {
		x = (x + (1+v)/x) / 2
	}
	return x
}
