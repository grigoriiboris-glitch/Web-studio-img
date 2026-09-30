
package visualdna

import (
	"image"
	"image/color"
	"testing"
)

func TestAnalyzeExtractsActualImageSignals(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 20, B: 20, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	got, err := Analyze("asset-1", img)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got.ID != "asset-1" || got.Width != 8 || got.Height != 4 {
		t.Fatalf("unexpected identity metadata: %+v", got)
	}
	if got.AspectRatio != 2 {
		t.Fatalf("AspectRatio = %v, want 2", got.AspectRatio)
	}
	if len(got.Palette) < 2 {
		t.Fatalf("Palette length = %d, want at least 2", len(got.Palette))
	}
	if got.Contrast <= 0 || got.EdgeDensity <= 0 {
		t.Fatalf("expected non-zero visual contrast/edges: %+v", got)
	}
	if got.Geometry.Dominant == "unknown" {
		t.Fatalf("expected geometry signal: %+v", got.Geometry)
	}
	if got.Lighting.Direction == "unknown" {
		t.Fatalf("expected lighting direction signal: %+v", got.Lighting)
	}
}

func TestBuildProfileCarriesAlgorithmAndSources(t *testing.T) {
	values := make([]AssetAnalysis, 0, 2)
	for i, brightness := range []uint8{40, 180} {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				img.SetRGBA(x, y, color.RGBA{R: brightness, G: brightness, B: brightness, A: 255})
			}
		}
		value, err := Analyze(string(rune('a'+i)), img)
		if err != nil {
			t.Fatalf("Analyze() error = %v", err)
		}
		values = append(values, value)
	}
	signals, summary := BuildProfile(values)
	if signals["algorithm_version"] != nil {
		t.Fatalf("unexpected top-level algorithm_version in signals")
	}
	brightness := signals["brightness"].(map[string]any)
	if brightness["algorithm"] != AlgorithmVersion {
		t.Fatalf("brightness algorithm = %v, want %s", brightness["algorithm"], AlgorithmVersion)
	}
	if len(brightness["source_assets"].([]string)) != 2 {
		t.Fatalf("brightness source assets = %#v", brightness["source_assets"])
	}
	if summary["uncertainty"] == "" {
		t.Fatalf("expected uncertainty")
	}
	if len(summary["recurring"].([]string)) == 0 {
		t.Fatalf("expected recurring summary for stable generated inputs")
	}
}

func TestCompareReportsProfileDifferences(t *testing.T) {
	values := make([]AssetAnalysis, 0, 2)
	for _, brightness := range []uint8{80, 100} {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				img.SetRGBA(x, y, color.RGBA{R: brightness, G: brightness, B: brightness, A: 255})
			}
		}
		value, err := Analyze("source", img)
		if err != nil {
			t.Fatalf("Analyze() error = %v", err)
		}
		values = append(values, value)
	}
	targetImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			targetImg.SetRGBA(x, y, color.RGBA{R: 220, G: 220, B: 220, A: 255})
		}
	}
	target, err := Analyze("target", targetImg)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	got := Compare(values, target)
	if len(got) == 0 {
		t.Fatal("Compare() returned no comparisons")
	}
	foundBrightness := false
	for _, item := range got {
		if item["signal"] == "brightness" {
			foundBrightness = true
			if item["delta"].(float64) <= 0 {
				t.Fatalf("expected target brighter than profile: %#v", item)
			}
		}
	}
	if !foundBrightness {
		t.Fatal("brightness comparison missing")
	}
}
