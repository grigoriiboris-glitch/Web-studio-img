package assistant

import (
	"math"
	"strings"
	"unicode"

	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
)

type criticAsset struct {
	ID         string
	StorageKey string
	Prompt     string
	HasAsset   bool
}

func criticObservation(dimension, observation, reason string, confidence float64) map[string]any {
	return map[string]any{
		"dimension":   dimension,
		"observation": observation,
		"reason":      reason,
		"confidence": clampConfidence(confidence),
	}
}

func buildCriticObservations(result similarity.Result, compA, compB map[string]any, promptA, promptB string) []map[string]any {
	observations := make([]map[string]any, 0, 12)

	addSimilarityObservation := func(dimension, label, reason string, score float64) {
		level := "low"
		confidence := 0.74
		switch {
		case score >= 0.8:
			level, confidence = "high", 0.90
		case score >= 0.5:
			level, confidence = "moderate", 0.82
		}
		observations = append(observations, criticObservation(
			dimension,
			label+" is "+level+" between the iterations.",
			reason,
			confidence,
		))
	}

	addSimilarityObservation("visual", "Visual similarity", "Perceptual hash and luminance histogram comparison.", result.Visual)
	addSimilarityObservation("composition", "Composition similarity", "Heuristic focal-center, edge-density, and aspect-ratio comparison.", result.Composition)
	addSimilarityObservation("semantic", "Semantic similarity proxy", "Image histogram/color embedding cosine similarity; this is a proxy, not a semantic model.", result.Semantic)
	addSimilarityObservation("style", "Style similarity", "Heuristic comparison of edge density, luminance variance, and histogram.", result.Style)

	aspectA, aspectB := numberFrom(compA["aspect_ratio"]), numberFrom(compB["aspect_ratio"])
	if aspectA > 0 && aspectB > 0 {
		delta := math.Abs(aspectB - aspectA)
		observations = append(observations, criticObservation(
			"composition",
			formatCompositionAspectObservation(delta),
			"Aspect ratios are compared from decoded image dimensions.",
			0.98,
		))
	}

	if focalA, focalB := focalPoint(compA), focalPoint(compB); focalA != nil && focalB != nil {
		distance := math.Hypot(focalA[0]-focalB[0], focalA[1]-focalB[1])
		observations = append(observations, criticObservation(
			"composition",
			formatFocalObservation(distance),
			"Estimated focal-point coordinates come from luminance-weighted image descriptors; they are not object detections.",
			0.72,
		))
	}

	if horizonA, okA := numberField(compA, "horizon"); okA {
		if horizonB, okB := numberField(compB, "horizon"); okB {
			delta := math.Abs(horizonB - horizonA)
			observations = append(observations, criticObservation(
				"composition",
				formatHorizonObservation(delta),
				"The stored horizon value is compared as a composition proxy produced by the deterministic analyzer.",
				0.70,
			))
		}
	}

	perspectiveA := stringField(compA, "perspective")
	perspectiveB := stringField(compB, "perspective")
	if perspectiveA != "" && perspectiveB != "" {
		observations = append(observations, criticObservation(
			"composition",
			formatPerspectiveObservation(perspectiveA, perspectiveB),
			"Perspective labels are deterministic aspect-ratio hints, not camera-geometry reconstruction.",
			0.70,
		))
	}

	if negativeA, okA := nestedNumberField(compA, "negative_space", "edge_density"); okA {
		if negativeB, okB := nestedNumberField(compB, "negative_space", "edge_density"); okB {
			delta := math.Abs(negativeB - negativeA)
			observations = append(observations, criticObservation(
				"composition",
				formatProxyDeltaObservation("Negative-space proxy", delta),
				"Negative-space change is inferred from edge-density difference; it is not segmentation.",
				0.68,
			))
		}
	}

	if scaleA, okA := nestedNumberField(compA, "object_scale", "heuristic_box_area"); okA {
		if scaleB, okB := nestedNumberField(compB, "object_scale", "heuristic_box_area"); okB {
			delta := math.Abs(scaleB - scaleA)
			observations = append(observations, criticObservation(
				"composition",
				formatProxyDeltaObservation("Estimated object-scale proxy", delta),
				"A fixed heuristic box area is compared; it does not represent detected object size.",
				0.60,
			))
		}
	}

	if boxA := boundingBoxCenter(compA); boxA != nil {
		if boxB := boundingBoxCenter(compB); boxB != nil {
			distance := math.Hypot(boxA[0]-boxB[0], boxA[1]-boxB[1])
			observations = append(observations, criticObservation(
				"composition",
				formatBoundingBoxObservation(distance),
				"Bounding-box centers come from heuristic image descriptors, not detected objects.",
				0.60,
			))
		}
	}

	if score, ok := promptSimilarity(promptA, promptB); ok {
		observations = append(observations, criticObservation(
			"prompt",
			formatPromptObservation(score),
			"Prompt similarity is a token-set Jaccard comparison of persisted prompt text.",
			0.95,
		))
	} else {
		observations = append(observations, criticObservation(
			"prompt",
			"Prompt comparison is unavailable.",
			"At least one iteration has no persisted prompt text, so no similarity score is inferred.",
			0.99,
		))
	}

	return observations
}

func promptSimilarity(a, b string) (float64, bool) {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return 0, false
	}
	return promptJaccard(a, b), true
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

func boundingBoxCenter(value map[string]any) []float64 {
	raw, ok := value["bounding_boxes"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	box, ok := raw[0].(map[string]any)
	if !ok {
		return nil
	}
	x0, ok0 := numberField(box, "x_min")
	y0, ok1 := numberField(box, "y_min")
	x1, ok2 := numberField(box, "x_max")
	y1, ok3 := numberField(box, "y_max")
	if !ok0 || !ok1 || !ok2 || !ok3 || x1 < x0 || y1 < y0 {
		return nil
	}
	return []float64{(x0 + x1) / 2, (y0 + y1) / 2}
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

func numberField(value map[string]any, key string) (float64, bool) {
	raw, ok := value[key]
	if !ok {
		return 0, false
	}
	switch raw.(type) {
	case float64, float32, int, int64:
		return numberFrom(raw), true
	default:
		return 0, false
	}
}

func nestedNumberField(value map[string]any, outer, inner string) (float64, bool) {
	raw, ok := value[outer].(map[string]any)
	if !ok {
		return 0, false
	}
	return numberField(raw, inner)
}

func stringField(value map[string]any, key string) string {
	raw, ok := value[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

func clampConfidence(value float64) float64 {
	switch {
	case math.IsNaN(value) || math.IsInf(value, 0):
		return 0
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
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

func formatHorizonObservation(delta float64) string {
	switch {
	case delta < 0.05:
		return "The horizon proxy is nearly unchanged."
	case delta < 0.2:
		return "The horizon proxy changed moderately."
	default:
		return "The horizon proxy changed substantially."
	}
}

func formatPerspectiveObservation(a, b string) string {
	if a == b {
		return "The perspective hint is unchanged."
	}
	return "The perspective hint changed from " + a + " to " + b + "."
}

func formatProxyDeltaObservation(label string, delta float64) string {
	switch {
	case delta < 0.05:
		return label + " is nearly unchanged."
	case delta < 0.2:
		return label + " changed moderately."
	default:
		return label + " changed substantially."
	}
}

func formatBoundingBoxObservation(distance float64) string {
	switch {
	case distance < 0.05:
		return "The estimated bounding-box center is nearly unchanged."
	case distance < 0.2:
		return "The estimated bounding-box center moved moderately."
	default:
		return "The estimated bounding-box center moved substantially."
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
