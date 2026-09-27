package similarity

import (
	"bytes"
	"fmt"
	"image"
	"math"

	_ "image/jpeg"
	_ "image/png"
)

const AlgorithmVersion = "1.0.0"

type Stats struct {
	Width, Height int
	AvgR, AvgG, AvgB float64
	Aspect, Variance, EdgeDensity float64
	CenterX, CenterY float64
	Hist [16]float64
	PHash uint64
	Embedding []float64
}

type Result struct {
	Visual, Composition, Semantic, Style float64
	PHashScore, HistogramScore, EmbeddingScore float64
	MetadataMatch, CompositionDescriptorScore float64
	Reference Stats
	Target Stats
}

func Analyze(referenceData, targetData []byte) (Result, error) {
	r, err := imageStats(referenceData)
	if err != nil {
		return Result{}, fmt.Errorf("reference image: %w", err)
	}
	t, err := imageStats(targetData)
	if err != nil {
		return Result{}, fmt.Errorf("target image: %w", err)
	}
	phash := pHashSimilarity(r.PHash, t.PHash)
	hist := clamp01(histCosine(r.Hist, t.Hist))
	embedding := clamp01(cosine(r.Embedding, t.Embedding))
	composition := clamp01((centerSimilarity(r, t) + (1 - math.Abs(r.EdgeDensity-t.EdgeDensity)) + aspectSimilarity(r.Aspect, t.Aspect)) / 3)
	style := clamp01(((1 - math.Abs(r.EdgeDensity-t.EdgeDensity)) + (1 - math.Abs(r.Variance-t.Variance)) + hist) / 3)
	metadata := aspectSimilarity(r.Aspect, t.Aspect)
	visual := clamp01((phash + hist) / 2)
	semantic := embedding
	return Result{
		Visual: visual, Composition: composition, Semantic: semantic, Style: style,
		PHashScore: phash, HistogramScore: hist, EmbeddingScore: embedding,
		MetadataMatch: metadata, CompositionDescriptorScore: composition,
		Reference: r, Target: t,
	}, nil
}

func Scores(referenceData, targetData []byte) (map[string]float64, error) {
	result, err := Analyze(referenceData, targetData)
	if err != nil {
		return nil, err
	}
	return map[string]float64{
		"visual": result.Visual,
		"composition": result.Composition,
		"semantic": result.Semantic,
		"style": result.Style,
		"color": clamp01((1 - math.Sqrt(sq(result.Reference.AvgR-result.Target.AvgR)+sq(result.Reference.AvgG-result.Target.AvgG)+sq(result.Reference.AvgB-result.Target.AvgB))/1.732)),
		"material": clamp01(1 - math.Abs(result.Reference.Variance-result.Target.Variance)),
		"geometry": result.MetadataMatch,
	}, nil
}

func imageStats(data []byte) (Stats, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Stats{}, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return Stats{}, fmt.Errorf("empty image")
	}
	var sr, sg, sb, sl, sl2, wx, wy float64
	var hist [16]float64
	var samples int
	var prev float64
	edges := 0
	sy, sx := maxInt(1, h/128), maxInt(1, w/128)
	grid := make([]float64, 0, 64)
	for y := 0; y < h; y += sy {
		for x := 0; x < w; x += sx {
			rr, gg, bb, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			rf, gf, bf := float64(rr)/65535, float64(gg)/65535, float64(bb)/65535
			l := 0.2126*rf + 0.7152*gf + 0.0722*bf
			sr += rf; sg += gf; sb += bf
			sl += l; sl2 += l*l
			idx := int(l * 16); if idx > 15 { idx = 15 }
			hist[idx]++
			nx := float64(x)/math.Max(1, float64(w-1))
			ny := float64(y)/math.Max(1, float64(h-1))
			wx += nx*l; wy += ny*l
			if samples > 0 && math.Abs(l-prev) > 0.12 { edges++ }
			prev = l
			samples++
		}
	}
	if samples == 0 {
		return Stats{}, fmt.Errorf("no samples")
	}
	mean := sl/float64(samples)
	for i := range hist { hist[i] /= float64(samples) }
	for gy := 0; gy < 8; gy++ {
		y := minInt(h-1, gy*h/8)
		for gx := 0; gx < 8; gx++ {
			x := minInt(w-1, gx*w/8)
			rr, gg, bb, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			l := 0.2126*float64(rr)/65535 + 0.7152*float64(gg)/65535 + 0.0722*float64(bb)/65535
			grid = append(grid, l)
		}
	}
	var avg float64
	for _, v := range grid { avg += v }
	avg /= float64(len(grid))
	var phash uint64
	for i, v := range grid { if v >= avg { phash |= uint64(1) << uint(i) } }
	embedding := append([]float64{}, hist[:]...)
	embedding = append(embedding, sr/float64(samples), sg/float64(samples), sb/float64(samples))
	embedding = append(embedding, mean, clamp01(sl2/float64(samples)-mean*mean), float64(edges)/float64(samples))
	embedding = append(embedding, wx/math.Max(sl,1e-9), wy/math.Max(sl,1e-9))
	return Stats{
		Width:w, Height:h, AvgR:sr/float64(samples), AvgG:sg/float64(samples), AvgB:sb/float64(samples),
		Aspect:float64(w)/float64(h), Variance:clamp01(sl2/float64(samples)-mean*mean),
		EdgeDensity:float64(edges)/float64(samples), CenterX:clamp01(wx/math.Max(sl,1e-9)), CenterY:clamp01(wy/math.Max(sl,1e-9)),
		Hist:hist, PHash:phash, Embedding:embedding,
	}, nil
}

func pHashSimilarity(a,b uint64) float64 {
	d := bitsCount(a^b)
	return 1 - float64(d)/64
}
func bitsCount(v uint64) int {
	n:=0
	for v != 0 { v &= v-1; n++ }
	return n
}
func cosine(a,b []float64) float64 {
	if len(a)==0 || len(a)!=len(b) { return 0 }
	var dot, aa, bb float64
	for i:=range a { dot+=a[i]*b[i]; aa+=a[i]*a[i]; bb+=b[i]*b[i] }
	if aa==0 || bb==0 { return 0 }
	return dot/math.Sqrt(aa*bb)
}
func centerSimilarity(a,b Stats) float64 { return clamp01(1-math.Hypot(a.CenterX-b.CenterX,a.CenterY-b.CenterY)/1.41421356237) }
func aspectSimilarity(a,b float64) float64 { return clamp01(1-math.Abs(a-b)/math.Max(a,b)) }
func histCosine(a,b [16]float64) float64 { var dot,aa,bb float64; for i:=range a {dot+=a[i]*b[i];aa+=a[i]*a[i];bb+=b[i]*b[i]}; if aa==0||bb==0{return 0};return dot/math.Sqrt(aa*bb) }
func sq(v float64) float64 { return v*v }
func clamp01(v float64) float64 { if v<0{return 0};if v>1{return 1};return v }
func maxInt(a,b int) int { if a>b{return a};return b }
func minInt(a,b int) int { if a<b{return a};return b }
