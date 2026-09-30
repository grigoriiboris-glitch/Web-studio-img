
package visualdna

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
)

const AlgorithmVersion = "visual-dna-v2.0"

type PaletteColor struct { Hex string; Share float64 }
type Lighting struct { Highlight, Shadow float64; Direction string; DirectionX, DirectionY float64 }
type Geometry struct { Dominant string; Orientation map[string]float64 }
type Texture struct { LocalVariance, HighFrequency float64 }
type Focal struct { X, Y float64; Grid [9]float64 }

type AssetAnalysis struct {
	ID string
	Width, Height int
	AspectRatio float64
	Palette []PaletteColor
	Brightness, Contrast, Temperature, Saturation float64
	Focal Focal
	EdgeDensity, DepthProxy float64
	Geometry Geometry
	Texture Texture
	Lighting Lighting
}

func Analyze(id string, img image.Image) (AssetAnalysis, error) {
	if img == nil { return AssetAnalysis{}, fmt.Errorf("image is nil") }
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 { return AssetAnalysis{}, fmt.Errorf("invalid image dimensions") }
	grid := sample(img)
	if len(grid.pixels) == 0 { return AssetAnalysis{}, fmt.Errorf("image has no pixels") }
	bright, contrast := meanStd(grid.pixels, func(p px) float64 { return p.b })
	sat, _ := meanStd(grid.pixels, func(p px) float64 { return p.s })
	temp, _ := meanStd(grid.pixels, func(p px) float64 { return p.t })
	focal, edges, geom, texture, light := spatial(grid)
	return AssetAnalysis{ID:id, Width:b.Dx(), Height:b.Dy(), AspectRatio:float64(b.Dx())/float64(b.Dy()), Palette:palette(grid.pixels), Brightness:bright, Contrast:contrast, Temperature:temp, Saturation:sat, Focal:focal, EdgeDensity:edges, DepthProxy:depth(grid), Geometry:geom, Texture:texture, Lighting:light}, nil
}

func (a AssetAnalysis) JSON() map[string]any {
	pal := make([]map[string]any, 0, len(a.Palette))
	for _, p := range a.Palette { pal = append(pal, map[string]any{"hex": p.Hex, "share": p.Share}) }
	grid := make([]float64, len(a.Focal.Grid))
	copy(grid, a.Focal.Grid[:])
	orient := map[string]float64{}
	for k,v := range a.Geometry.Orientation { orient[k]=v }
	return map[string]any{
		"asset_id":a.ID, "width":a.Width, "height":a.Height, "aspect_ratio":a.AspectRatio,
		"palette":pal, "brightness":a.Brightness, "contrast":a.Contrast,
		"color_temperature":a.Temperature, "saturation":a.Saturation,
		"focal_distribution":map[string]any{"center_x":a.Focal.X,"center_y":a.Focal.Y,"grid_3x3":grid},
		"edge_density":a.EdgeDensity, "depth_proxy":a.DepthProxy,
		"dominant_geometry":map[string]any{"dominant":a.Geometry.Dominant,"orientation_share":orient},
		"material_texture_indicators":map[string]any{"local_variance":a.Texture.LocalVariance,"high_frequency":a.Texture.HighFrequency},
		"lighting_indicators":map[string]any{"highlight_ratio":a.Lighting.Highlight,"shadow_ratio":a.Lighting.Shadow,"direction":a.Lighting.Direction,"direction_x":a.Lighting.DirectionX,"direction_y":a.Lighting.DirectionY},
	}
}

func BuildProfile(values []AssetAnalysis) (map[string]any, map[string]any) {
	if len(values)==0 { return map[string]any{}, map[string]any{} }
	ids:=make([]string,0,len(values))
	for _,v:=range values { ids=append(ids,v.ID) }
	agg:=func(name string, f func(AssetAnalysis) float64) map[string]any {
		s:=make([]float64,0,len(values))
		for _,v:=range values{s=append(s,f(v))}
		m,sd:=mean(s)
		return map[string]any{"mean":m,"std_dev":sd,"min":min(s),"max":max(s),"confidence":confidence(len(values),sd),"source_assets":ids,"algorithm":AlgorithmVersion,"uncertainty":uncertainty(len(values))}
	}
	pal:=map[string]float64{}
	geom:=map[string]float64{}
	dir:=map[string]float64{}
	var fx,fy,hl,sh float64
	for _,v:=range values{
		for _,p:=range v.Palette{pal[p.Hex]+=p.Share/float64(len(values))}
		geom[v.Geometry.Dominant]+=1/float64(len(values))
		dir[v.Lighting.Direction]+=1/float64(len(values))
		fx+=v.Focal.X;fy+=v.Focal.Y;hl+=v.Lighting.Highlight;sh+=v.Lighting.Shadow
	}
	type ranked struct{hex string;share float64}
	r:=make([]ranked,0,len(pal));for h,s:=range pal{r=append(r,ranked{h,s})}
	sort.Slice(r,func(i,j int)bool{return r[i].share>r[j].share})
	palette:=make([]map[string]any,0,minInt(8,len(r)));for _,x:=range r[:minInt(8,len(r))]{palette=append(palette,map[string]any{"hex":x.hex,"share":x.share})}
	brightness:=agg("brightness",func(v AssetAnalysis)float64{return v.Brightness})
	contrast:=agg("contrast",func(v AssetAnalysis)float64{return v.Contrast})
	temp:=agg("color_temperature",func(v AssetAnalysis)float64{return v.Temperature})
	sat:=agg("saturation",func(v AssetAnalysis)float64{return v.Saturation})
	edges:=agg("edge_density",func(v AssetAnalysis)float64{return v.EdgeDensity})
	depthP:=agg("depth_proxy",func(v AssetAnalysis)float64{return v.DepthProxy})
	texture:=agg("material_texture",func(v AssetAnalysis)float64{return v.Texture.HighFrequency})
	aspect:=agg("aspect_ratio",func(v AssetAnalysis)float64{return v.AspectRatio})
	recurring:=[]string{}
	for _,x:=range []struct{name string;signal map[string]any}{
		{"brightness",brightness},{"contrast",contrast},{"color_temperature",temp},{"saturation",sat},{"edge_density",edges},{"depth_proxy",depthP},{"material_texture",texture},
	}{if x.signal["std_dev"].(float64)<=0.12{recurring=append(recurring,x.name)}}
	emerging:=emergingSignals(values)
	outliers:=outliers(values,map[string]map[string]any{"brightness":brightness,"contrast":contrast,"saturation":sat,"edge_density":edges,"depth_proxy":depthP,"material_texture":texture})
	fc:=(confidence(len(values),0.0)+confidence(len(values),contrast["std_dev"].(float64))+confidence(len(values),sat["std_dev"].(float64)))/3
	unc:="Deterministic image descriptors. Depth, material and lighting semantics are proxies; no prompt mutation is performed."
	if len(values)==1{unc="Single source asset; recurring, emerging and outlier conclusions are not statistically stable."}
	signals:=map[string]any{
		"algorithm_version": AlgorithmVersion,
		"palette":map[string]any{"value":palette,"source_assets":ids,"algorithm":AlgorithmVersion,"confidence":confidence(len(values),0),"uncertainty":unc},
		"aspect_ratio":aspect,"brightness":brightness,"contrast":contrast,"color_temperature":temp,"saturation":sat,
		"focal_distribution":map[string]any{"center_x":fx/float64(len(values)),"center_y":fy/float64(len(values)),"algorithm":AlgorithmVersion,"source_assets":ids,"confidence":confidence(len(values),0.1),"uncertainty":uncertainty(len(values))},
		"edge_density":edges,"depth_proxy":depthP,
		"dominant_geometry":map[string]any{"value":geom,"source_assets":ids,"algorithm":AlgorithmVersion,"confidence":confidence(len(values),0.1),"uncertainty":"Inferred from edge orientations."},
		"material_texture":texture,
		"lighting":map[string]any{"highlight_ratio":hl/float64(len(values)),"shadow_ratio":sh/float64(len(values)),"directions":dir,"algorithm":AlgorithmVersion,"source_assets":ids,"confidence":confidence(len(values),0.1),"uncertainty":uncertainty(len(values))},
		"asset_values":func()[]map[string]any{out:=make([]map[string]any,0,len(values));for _,v:=range values{out=append(out,v.JSON())};return out}(),
	}
	summary:=map[string]any{"recurring":recurring,"emerging":emerging,"outliers":outliers,"confidence":fc,"uncertainty":unc}
	return signals,summary
}

func Compare(values []AssetAnalysis,target AssetAnalysis) []map[string]any {
	pairs:=[]struct{name string;f func(AssetAnalysis)float64;v float64}{
		{"brightness",func(v AssetAnalysis)float64{return v.Brightness},target.Brightness},
		{"contrast",func(v AssetAnalysis)float64{return v.Contrast},target.Contrast},
		{"color_temperature",func(v AssetAnalysis)float64{return v.Temperature},target.Temperature},
		{"saturation",func(v AssetAnalysis)float64{return v.Saturation},target.Saturation},
		{"edge_density",func(v AssetAnalysis)float64{return v.EdgeDensity},target.EdgeDensity},
		{"depth_proxy",func(v AssetAnalysis)float64{return v.DepthProxy},target.DepthProxy},
		{"material_texture",func(v AssetAnalysis)float64{return v.Texture.HighFrequency},target.Texture.HighFrequency},
	}
	out:=make([]map[string]any,0,len(pairs))
	for _,p:=range pairs{m,_:=seriesMean(values,p.f);d:=p.v-m;i:="close to profile";if math.Abs(d)>=0.15{i="noticeably different"}else if math.Abs(d)>=0.08{i="slightly different"};out=append(out,map[string]any{"signal":p.name,"profile_mean":m,"asset_value":p.v,"delta":d,"interpretation":i})}
	return out
}

func emergingSignals(v []AssetAnalysis) []string {
	if len(v)<4{return []string{}}
	mid:=len(v)/2
	type c struct{name string;delta float64}
	candidates:=[]c{}
	add:=func(name string,f func(AssetAnalysis)float64){a,_:=seriesMean(v[:mid],f);b,_:=seriesMean(v[mid:],f);candidates=append(candidates,c{name,b-a})}
	add("brightness",func(x AssetAnalysis)float64{return x.Brightness});add("contrast",func(x AssetAnalysis)float64{return x.Contrast});add("saturation",func(x AssetAnalysis)float64{return x.Saturation});add("edge_density",func(x AssetAnalysis)float64{return x.EdgeDensity});add("material_texture",func(x AssetAnalysis)float64{return x.Texture.HighFrequency})
	sort.Slice(candidates,func(i,j int)bool{return math.Abs(candidates[i].delta)>math.Abs(candidates[j].delta)})
	out:=[]string{};for _,x:=range candidates{if math.Abs(x.delta)>=0.08{out=append(out,fmt.Sprintf("%s:%+.2f",x.name,x.delta))};if len(out)==3{break}}
	return out
}

func outliers(values []AssetAnalysis, aggs map[string]map[string]any) []map[string]any {
	type score struct{id string;d float64};items:=[]score{}
	for _,a:=range values{d:=0.0;for name,agg:=range aggs{m:=agg["mean"].(float64);sd:=math.Max(agg["std_dev"].(float64),0.05);d+=math.Abs(metric(a,name)-m)/sd};d/=float64(len(aggs));if d>=1.75{items=append(items,score{a.ID,d})}}
	sort.Slice(items,func(i,j int)bool{return items[i].d>items[j].d});if len(items)>5{items=items[:5]}
	out:=make([]map[string]any,0,len(items));for _,x:=range items{out=append(out,map[string]any{"asset_id":x.id,"distance":x.d,"reason":"Combined deviation across numeric visual signals."})};return out
}

func metric(a AssetAnalysis,name string)float64{switch name{case "brightness":return a.Brightness;case "contrast":return a.Contrast;case "saturation":return a.Saturation;case "edge_density":return a.EdgeDensity;case "depth_proxy":return a.DepthProxy;case "material_texture":return a.Texture.HighFrequency};return 0}

func sample(img image.Image) grid {
	b:=img.Bounds();step:=1;maxDim:=256;if b.Dx()>maxDim||b.Dy()>maxDim{step=int(math.Ceil(float64(maxInt(b.Dx(),b.Dy()))/float64(maxDim)))}
	w:=(b.Dx()+step-1)/step;h:=(b.Dy()+step-1)/step;pxs:=make([]px,0,w*h)
	for y:=b.Min.Y;y<b.Max.Y;y+=step{for x:=b.Min.X;x<b.Max.X;x+=step{c:=color.NRGBAModel.Convert(img.At(x,y)).(color.NRGBA);r,g,bl:=float64(c.R)/255,float64(c.G)/255,float64(c.B)/255;br:=0.2126*r+0.7152*g+0.0722*bl;mx,mn:=max3(r,g,bl),min3(r,g,bl);s:=0.0;if mx>0{s=(mx-mn)/mx};pxs=append(pxs,px{r:r,g:g,b:bl,lum:br,s:s,t:(r-bl)/(r+bl+0.0001)})}}
	return grid{w:w,h:h,pixels:pxs}
}

type px struct{r,g,b,lum,s,t float64}
type grid struct{w,h int;pixels []px}

func palette(p []px) []PaletteColor {type bucket struct{n int;r,g,b float64};m:=map[[3]int]*bucket{};for _,x:=range p{k:=[3]int{int(x.r*7+0.5),int(x.g*7+0.5),int(x.b*7+0.5)};if m[k]==nil{m[k]=&bucket{}};m[k].n++;m[k].r+=x.r;m[k].g+=x.g;m[k].b+=x.b};type rnk struct{n int;r,g,b float64};a:=make([]rnk,0,len(m));for _,x:=range m{a=append(a,rnk{x.n,x.r/float64(x.n),x.g/float64(x.n),x.b/float64(x.n)})};sort.Slice(a,func(i,j int)bool{return a[i].n>a[j].n});out:=make([]PaletteColor,0,minInt(6,len(a)));for _,x:=range a[:minInt(6,len(a))]{out=append(out,PaletteColor{fmt.Sprintf("#%02X%02X%02X",int(x.r*255),int(x.g*255),int(x.b*255)),float64(x.n)/float64(len(p))})};return out}

func spatial(g grid)(Focal,float64,Geometry,Texture,Lighting){
	if g.w<3||g.h<3{return Focal{},0,Geometry{"unknown",map[string]float64{}},Texture{},Lighting{Direction:"unknown"}}
	idx:=func(x,y int)int{return y*g.w+x};ori:=map[string]float64{"horizontal":0,"vertical":0,"diagonal":0};var sum,sx,sy,edges,varSum,hf,hi,lo,bx,by,bw float64
	for y:=1;y<g.h-1;y++{for x:=1;x<g.w-1;x++{p:=g.pixels[idx(x,y)];l:=g.pixels[idx(x-1,y)].lum;r:=g.pixels[idx(x+1,y)].lum;t:=g.pixels[idx(x,y-1)].lum;b:=g.pixels[idx(x,y+1)].lum;gx,gy:=r-l,b-t;mag:=math.Hypot(gx,gy);varSum+=(gx*gx+gy*gy)/2;hf+=mag;if mag>0.10{edges++};sal:=mag*(0.35+0.65*p.s);sum+=sal;sx+=float64(x)/float64(g.w-1)*sal;sy+=float64(y)/float64(g.h-1)*sal;if p.lum>=0.92{hi++};if p.lum<=0.08{lo++};if p.lum>=0.85{w:=p.lum-0.8;bw+=w;bx+=float64(x)/float64(g.w-1)*w;by+=float64(y)/float64(g.h-1)*w};ang:=math.Abs(math.Atan2(gy,gx));if ang<math.Pi/8||ang>=7*math.Pi/8{ori["horizontal"]+=mag}else if ang>=3*math.Pi/8&&ang<5*math.Pi/8{ori["vertical"]+=mag}else{ori["diagonal"]+=mag}}}
	q:=[9]float64{};for y:=1;y<g.h-1;y++{for x:=1;x<g.w-1;x++{p:=g.pixels[idx(x,y)];l:=g.pixels[idx(x-1,y)].lum;r:=g.pixels[idx(x+1,y)].lum;t:=g.pixels[idx(x,y-1)].lum;b:=g.pixels[idx(x,y+1)].lum;sal:=math.Hypot(r-l,b-t)*(0.35+0.65*p.s);col:=minInt(2,int(float64(x)/float64(g.w)*3));row:=minInt(2,int(float64(y)/float64(g.h)*3));q[row*3+col]+=sal}}
	var qs float64;for _,v:=range q{qs+=v};if qs>0{for i:=range q{q[i]/=qs}};cx,cy:=0.5,0.5;if sum>0{cx=sx/sum;cy=sy/sum};tot,mx:=0.0,0.0;dom:="mixed";for _,v:=range ori{tot+=v;if v>mx{mx=v}};if tot>0{for k:=range ori{ori[k]/=tot};for k,v:=range ori{if v>=0.45{dom=k;break}}}
	dir:="unknown";dx,dy:=0.0,0.0;if bw>0{cx2,cy2:=bx/bw,by/bw;dx,dy=cx2-0.5,cy2-0.5;switch{case math.Abs(dx)>math.Abs(dy)&&dx<-.12:dir="left";case math.Abs(dx)>math.Abs(dy)&&dx>.12:dir="right";case math.Abs(dy)>=math.Abs(dx)&&dy<-.12:dir="top";case math.Abs(dy)>=math.Abs(dx)&&dy>.12:dir="bottom";default:dir="center"}}
	area:=float64((g.w-2)*(g.h-2));return Focal{cx,cy,q},edges/area,Geometry{dom,ori},Texture{varSum/area,hf/area},Lighting{hi/float64(g.w*g.h),lo/float64(g.w*g.h),dir,dx,dy}
}

func depth(g grid)float64{if g.w<3||g.h<3{return 0.5};bands:=[3]float64{};for y:=0;y<g.h;y++{band:=0;if y>=g.h/3{band=1};if y>=2*g.h/3{band=2};for x:=1;x<g.w-1;x++{bands[band]+=math.Abs(g.pixels[y*g.w+x+1].lum-g.pixels[y*g.w+x-1].lum)}};front:=bands[2]/float64(maxInt(1,g.w-2));back:=bands[0]/float64(maxInt(1,g.w-2));return clamp(0.5+0.5*math.Tanh((front-back)*4),0,1)}

func meanStd(p []px,f func(px)float64)(float64,float64){a:=make([]float64,0,len(p));for _,x:=range p{a=append(a,f(x))};return mean(a)}
func mean(a []float64)(float64,float64){if len(a)==0{return 0,0};var s float64;for _,v:=range a{s+=v};m:=s/float64(len(a));var q float64;for _,v:=range a{d:=v-m;q+=d*d};return m,math.Sqrt(q/float64(len(a)))}
func seriesMean(a []AssetAnalysis,f func(AssetAnalysis)float64)(float64,float64){x:=make([]float64,0,len(a));for _,v:=range a{x=append(x,f(v))};return mean(x)}
func min(a []float64)float64{if len(a)==0{return 0};m:=a[0];for _,v:=range a[1:]{if v<m{m=v}};return m}
func max(a []float64)float64{if len(a)==0{return 0};m:=a[0];for _,v:=range a[1:]{if v>m{m=v}};return m}
func confidence(n int,sd float64)float64{v:=1-math.Min(sd*2,0.7);if n<2{v*=0.5}else if n<4{v*=0.8};return clamp(v,0.05,0.99)}
func uncertainty(n int)string{if n<2{return "Single source asset; distribution uncertainty is high."};if n<4{return "Small sample; recurring and outlier conclusions are directional."};return ""}
func clamp(v,a,b float64)float64{if v<a{return a};if v>b{return b};return v}
func max3(a,b,c float64)float64{if a>b{if a>c{return a};return c};if b>c{return b};return c}
func min3(a,b,c float64)float64{if a<b{if a<c{return a};return c};if b<c{return b};return c}
func minInt(a,b int)int{if a<b{return a};return b}
func maxInt(a,b int)int{if a>b{return a};return b}
