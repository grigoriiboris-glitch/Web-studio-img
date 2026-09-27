package similarity

import (
 "bytes"
 "fmt"
 "image"
 "math"
)

type Stats struct { AvgR,AvgG,AvgB float64; Aspect,Variance,EdgeDensity float64; CenterX,CenterY float64; Hist [16]float64 }
func Scores(referenceData,targetData []byte)(map[string]float64,error){r,err:=imageStats(referenceData);if err!=nil{return nil,fmt.Errorf("reference image: %w",err)};t,err:=imageStats(targetData);if err!=nil{return nil,fmt.Errorf("target image: %w",err)};return map[string]float64{
 "composition":clamp01((1-math.Hypot(r.CenterX-t.CenterX,r.CenterY-t.CenterY)/1.4142+1-math.Abs(r.EdgeDensity-t.EdgeDensity))/2),
 "semantic":clamp01(histCosine(r.Hist,t.Hist)),
 "color":clamp01(1-math.Sqrt(sq(r.AvgR-t.AvgR)+sq(r.AvgG-t.AvgG)+sq(r.AvgB-t.AvgB))/1.732),
 "style":clamp01(1-math.Abs(r.EdgeDensity-t.EdgeDensity)),
 "material":clamp01(1-math.Abs(r.Variance-t.Variance)),
 "geometry":clamp01(1-math.Abs(r.Aspect-t.Aspect)/math.Max(r.Aspect,t.Aspect)),
},nil}
func imageStats(data []byte)(Stats,error){img,_,err:=image.Decode(bytes.NewReader(data));if err!=nil{return Stats{},err};b:=img.Bounds();w,h:=b.Dx(),b.Dy();if w<=0||h<=0{return Stats{},fmt.Errorf("empty image")};var sr,sg,sb,sl,sl2,wx,wy float64;var hist [16]float64;var prev float64;edges,samples:=0,0;sy:=maxInt(1,h/128);sx:=maxInt(1,w/128);for y:=0;y<h;y+=sy{for x:=0;x<w;x+=sx{rr,gg,bb,_:=img.At(b.Min.X+x,b.Min.Y+y).RGBA();rf,gf,bf:=float64(rr)/65535,float64(gg)/65535,float64(bb)/65535;l:=0.2126*rf+0.7152*gf+0.0722*bf;sr+=rf;sg+=gf;sb+=bf;sl+=l;sl2+=l*l;idx:=int(l*16);if idx>15{idx=15};hist[idx]++;nx:=float64(x)/math.Max(1,float64(w-1));ny:=float64(y)/math.Max(1,float64(h-1));wx+=nx*l;wy+=ny*l;if x>0&&math.Abs(l-prev)>0.12{edges++};prev=l;samples++}};if samples==0{return Stats{},fmt.Errorf("no samples")};mean:=sl/float64(samples);for i:=range hist{hist[i]/=float64(samples)};return Stats{AvgR:sr/float64(samples),AvgG:sg/float64(samples),AvgB:sb/float64(samples),Aspect:float64(w)/float64(h),Variance:clamp01(sl2/float64(samples)-mean*mean),EdgeDensity:float64(edges)/float64(samples),CenterX:clamp01(wx/math.Max(sl,1e-9)),CenterY:clamp01(wy/math.Max(sl,1e-9)),Hist:hist},nil}
func histCosine(a,b [16]float64)float64{var dot,aa,bb float64;for i:=range a{dot+=a[i]*b[i];aa+=a[i]*a[i];bb+=b[i]*b[i]};if aa==0||bb==0{return 0};return dot/math.Sqrt(aa*bb)}
func sq(v float64)float64{return v*v}
func clamp01(v float64)float64{if v<0{return 0};if v>1{return 1};return v}
func maxInt(a,b int)int{if a>b{return a};return b}
