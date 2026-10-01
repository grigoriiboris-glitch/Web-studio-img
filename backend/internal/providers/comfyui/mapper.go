package comfyui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type mapperValues struct {
	Prompt string
	NegativePrompt string
	Seed *int64
	AspectRatio string
	InputImage string
	MaskImage string
	Width int
	Height int
	Operation string
	Assets map[string]string
}

func mapWorkflow(template map[string]any, values mapperValues) (map[string]any, error) {
	encoded, err := json.Marshal(template)
	if err != nil { return nil, fmt.Errorf("encode workflow: %w", err) }
	var value any
	if err := json.Unmarshal(encoded,&value); err != nil { return nil, fmt.Errorf("decode workflow: %w",err) }
	mapped:=replace(value,values)
	out,ok:=mapped.(map[string]any)
	if !ok{return nil,fmt.Errorf("workflow must be a JSON object")}
	return out,nil
}

func replace(value any,v mapperValues) any {
	switch x:=value.(type){
	case map[string]any:
		for k,item:=range x{x[k]=replace(item,v)}
		return x
	case []any:
		for i,item:=range x{x[i]=replace(item,v)}
		return x
	case string:
		return replaceString(x,v)
	default:
		return value
	}
}

func replaceString(s string,v mapperValues) any {
	values:=map[string]string{
		"{{prompt}}":v.Prompt,
		"{{negative_prompt}}":v.NegativePrompt,
		"{{input_image}}":v.InputImage,
		"{{mask_image}}":v.MaskImage,
		"{{aspect_ratio}}":v.AspectRatio,
		"{{operation}}":v.Operation,
		"{{width}}":strconv.Itoa(v.Width),
		"{{height}}":strconv.Itoa(v.Height),
	}
	for key,name:=range v.Assets { values["{{asset:"+key+"}}"]=name }
	if v.Seed!=nil{values["{{seed}}"]=strconv.FormatInt(*v.Seed,10)}else{values["{{seed}}"]="0"}
	if val,ok:=values[s];ok{
		if n,err:=strconv.ParseInt(val,10,64);err==nil&&strings.HasPrefix(s,"{{")&&(s=="{{seed}}"||s=="{{width}}"||s=="{{height}}"){return n}
		return val
	}
	for token,val:=range values{s=strings.ReplaceAll(s,token,val)}
	return s
}
