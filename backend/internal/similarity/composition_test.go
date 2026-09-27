package similarity

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func compositionTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0,0,16,8))
	for y:=0;y<8;y++{for x:=0;x<16;x++{v:=uint8((x+y)*8);img.Set(x,y,color.RGBA{v,v,v,255})}}
	var b bytes.Buffer
	_ = png.Encode(&b,img)
	return b.Bytes()
}

func TestAnalyzeCompositionIsDeterministicAndHeuristic(t *testing.T) {
	data:=compositionTestPNG()
	a,err:=AnalyzeComposition(data);if err!=nil{t.Fatal(err)}
	b,err:=AnalyzeComposition(data);if err!=nil{t.Fatal(err)}
	if a["analysis_mode"]!="deterministic-image-descriptors"{t.Fatalf("unexpected mode: %v",a["analysis_mode"])}
	if a["horizon"]!=b["horizon"]{t.Fatal("horizon changed")}
	if a["camera_elevation"]!=nil{t.Fatal("camera elevation must remain unknown in MVP")}
}
