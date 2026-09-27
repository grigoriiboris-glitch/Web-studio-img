package similarity

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngImage(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAnalyzeIdenticalImagesProducesIndependentScores(t *testing.T) {
	data := pngImage(t, color.RGBA{R: 200, G: 120, B: 80, A: 255})
	result, err := Analyze(data, data)
	if err != nil {
		t.Fatal(err)
	}
	for name, score := range map[string]float64{
		"visual": result.Visual,
		"composition": result.Composition,
		"semantic": result.Semantic,
		"style": result.Style,
		"p_hash": result.PHashScore,
		"histogram": result.HistogramScore,
		"embedding": result.EmbeddingScore,
	} {
		if score < 0 || score > 1 {
			t.Fatalf("%s score out of range: %f", name, score)
		}
	}
	if result.Visual != 1 || result.PHashScore != 1 || result.HistogramScore != 1 || result.EmbeddingScore != 1 {
		t.Fatalf("identical image scores are not stable: %#v", result)
	}
}

func TestScoresReturnsAllInfluenceDimensions(t *testing.T) {
	reference := pngImage(t, color.RGBA{R: 240, G: 240, B: 240, A: 255})
	target := pngImage(t, color.RGBA{R: 32, G: 64, B: 96, A: 255})
	scores, err := Scores(reference, target)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"visual", "composition", "semantic", "style", "color", "material", "geometry"} {
		score, ok := scores[name]
		if !ok {
			t.Fatalf("missing score %q", name)
		}
		if score < 0 || score > 1 {
			t.Fatalf("score %q out of range: %f", name, score)
		}
	}
	if AlgorithmVersion == "" {
		t.Fatal("algorithm version must be non-empty")
	}
}
