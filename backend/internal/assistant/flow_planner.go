package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/providers/comfyui"
)

type FlowInput struct {
	ID string
	Type string
	Label string
	Required bool
}

type FlowPlan struct {
	Name string
	Description string
	Prompt string
	NegativePrompt string
	Workflow map[string]any
	Parameters map[string]any
	Inputs []FlowInput
	SelectedModel map[string]string
	Reasoning string
	ArtistSteps []map[string]any
}

type FlowPlanner struct {
	BaseURL string
	APIKey string
	Model string
	Client *http.Client
}

func NewFlowPlannerFromEnv() *FlowPlanner {
	return &FlowPlanner{
		BaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("FLOW_PLANNER_BASE_URL")), "/"),
		APIKey: strings.TrimSpace(os.Getenv("FLOW_PLANNER_API_KEY")),
		Model: strings.TrimSpace(os.Getenv("FLOW_PLANNER_MODEL")),
		Client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (p *FlowPlanner) Enabled() bool {
	return p != nil && p.BaseURL != "" && p.Model != "" && p.Client != nil
}

func (p *FlowPlanner) Plan(ctx context.Context, task string, snapshot comfyui.RuntimeCapabilitySnapshot, validationErrors []string, currentWorkflow map[string]any) (FlowPlan, error) {
	if !p.Enabled() {
		return FlowPlan{}, errors.New("FLOW_PLANNER_BASE_URL and FLOW_PLANNER_MODEL must be configured")
	}
	task = strings.TrimSpace(task)
	if task == "" {
		return FlowPlan{}, errors.New("task is required")
	}
	system := "You are a ComfyUI workflow engineer. Generate only API-format workflows using node types and model files explicitly present in the runtime snapshot. Never invent nodes, models, paths, URLs or credentials. Use exact class_type names and model filenames. Use placeholders {{prompt}}, {{negative_prompt}}, {{seed}}, {{width}}, {{height}}, {{input_image}}, {{mask_image}} and {{asset:<input_id>}}. For external images declare a stable input id and use its asset placeholder in the workflow. Return only JSON."
	user := map[string]any{
		"task": task,
		"runtime": compactRuntime(task, snapshot),
		"previous_validation_errors": validationErrors,
		"current_workflow": currentWorkflow,
		"output_schema": map[string]any{
			"name": "string",
			"description": "string",
			"prompt": "string",
			"negative_prompt": "string",
			"inputs": []map[string]any{{"id":"sketch","type":"image","label":"Sketch","required":true}},
			"parameters": map[string]any{"width":1024,"height":1024},
			"selected_model": map[string]string{"folder":"checkpoints","filename":"exact installed filename"},
			"workflow": map[string]any{"1":map[string]any{"class_type":"ExactNode","inputs":map[string]any{}}},
			"reasoning": "short explanation",
			"artist_steps": []map[string]any{{"id":"sketch","enabled":true,"order":0,"evidence":[]map[string]any{{"kind":"asset","value":"sketch"}}}},
		},
	}
	payload := map[string]any{
		"model": p.Model,
		"temperature": 0,
		"messages": []map[string]any{
			{"role":"system","content":system},
			{"role":"user","content":mustJSONString(user)},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return FlowPlan{}, fmt.Errorf("encode planner request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx,http.MethodPost,p.BaseURL+"/chat/completions",bytes.NewReader(data))
	if err != nil {
		return FlowPlan{}, err
	}
	req.Header.Set("Content-Type","application/json")
	if p.APIKey != "" { req.Header.Set("Authorization","Bearer "+p.APIKey) }
	resp, err := p.Client.Do(req)
	if err != nil { return FlowPlan{}, fmt.Errorf("flow planner unavailable: %w",err) }
	defer func(){ _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf := make([]byte,16<<10)
		n,_ := resp.Body.Read(buf)
		return FlowPlan{}, fmt.Errorf("flow planner returned status %d: %s",resp.StatusCode,strings.TrimSpace(string(buf[:n])))
	}
	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return FlowPlan{}, fmt.Errorf("decode planner response: %w",err)
	}
	choices,_ := response["choices"].([]any)
	if len(choices)==0 { return FlowPlan{},errors.New("flow planner returned an empty response") }
	choice,_ := choices[0].(map[string]any)
	message,_ := choice["message"].(map[string]any)
	content,_ := message["content"].(string)
	if strings.TrimSpace(content)=="" { return FlowPlan{},errors.New("flow planner returned an empty response") }
	return decodeFlowPlan(content)
}

func decodeFlowPlan(content string) (FlowPlan,error) {
	content=strings.TrimSpace(content)
	if strings.HasPrefix(content,"```") {
		content=strings.TrimPrefix(content,"```json")
		content=strings.TrimPrefix(content,"```JSON")
		content=strings.TrimPrefix(content,"```")
		content=strings.TrimSuffix(strings.TrimSpace(content),"```")
	}
	var raw map[string]any
	if err:=json.Unmarshal([]byte(content),&raw);err!=nil {
		return FlowPlan{},fmt.Errorf("planner returned invalid JSON: %w",err)
	}
	name,_:=raw["name"].(string)
	workflow,_:=raw["workflow"].(map[string]any)
	if strings.TrimSpace(name)==""||len(workflow)==0 { return FlowPlan{},errors.New("planner returned an incomplete flow plan") }
	parameters,_:=raw["parameters"].(map[string]any)
	if parameters==nil { parameters=map[string]any{} }
	selectedRaw,_:=raw["selected_model"].(map[string]any)
	selected:=map[string]string{}
	for key,value:=range selectedRaw { if text,ok:=value.(string);ok {selected[key]=text} }
	inputs:=[]FlowInput{}
	if rawInputs,ok:=raw["inputs"].([]any);ok {
		for _,rawInput:=range rawInputs {
			object,_:=rawInput.(map[string]any)
			id,_:=object["id"].(string);kind,_:=object["type"].(string);label,_:=object["label"].(string);required,_:=object["required"].(bool)
			if strings.TrimSpace(id)==""||strings.TrimSpace(kind)=="" { return FlowPlan{},errors.New("planner returned an invalid flow input") }
			inputs=append(inputs,FlowInput{ID:strings.TrimSpace(id),Type:strings.TrimSpace(kind),Label:strings.TrimSpace(label),Required:required})
		}
	}
	artistSteps,_:=raw["artist_steps"].([]any)
	getString:=func(key string) string { value,_:=raw[key].(string);return strings.TrimSpace(value) }
	return FlowPlan{Name:strings.TrimSpace(name),Description:getString("description"),Prompt:getString("prompt"),NegativePrompt:getString("negative_prompt"),Workflow:workflow,Parameters:parameters,Inputs:inputs,SelectedModel:selected,Reasoning:getString("reasoning"),ArtistSteps:artistSteps},nil
}

func compactRuntime(task string,snapshot comfyui.RuntimeCapabilitySnapshot) map[string]any {
	keywords:=taskKeywords(task)
	nodeNames:=make([]string,0,len(snapshot.Nodes))
	for name:=range snapshot.Nodes { nodeNames=append(nodeNames,name) }
	sort.Slice(nodeNames,func(i,j int)bool{a,b:=nodeScore(nodeNames[i],keywords),nodeScore(nodeNames[j],keywords);if a!=b{return a>b};return nodeNames[i]<nodeNames[j]})
	if len(nodeNames)>120 {nodeNames=nodeNames[:120]}
	schemas:=map[string]any{}
	for _,name:=range nodeNames {info:=snapshot.Nodes[name];schemas[name]=map[string]any{"category":info.Category,"required_inputs":info.RequiredInputs,"optional_inputs":info.OptionalInputs,"outputs":info.OutputTypes}}
	models:=map[string]any{}
	folders:=make([]string,0,len(snapshot.Models))
	for folder:=range snapshot.Models {folders=append(folders,folder)}
	sort.Strings(folders)
	preferred:=map[string]bool{"checkpoints":true,"diffusion_models":true,"unet":true,"vae":true,"clip":true,"loras":true,"controlnet":true,"ipadapter":true,"clip_vision":true,"style_models":true,"upscale_models":true}
	for _,folder:=range folders {
		names:=append([]string(nil),snapshot.Models[folder]...)
		sort.Slice(names,func(i,j int)bool{a,b:=stringScore(names[i],keywords),stringScore(names[j],keywords);if a!=b{return a>b};return names[i]<names[j]})
		limit:=25;if preferred[folder]{limit=80};if len(names)>limit{names=names[:limit]}
		models[folder]=names
	}
	return map[string]any{"system":snapshot.System,"node_types":nodeNames,"node_schemas":schemas,"models":models}
}

func taskKeywords(task string) []string {
	fields:=strings.Fields(strings.ToLower(task));out:=make([]string,0,len(fields))
	for _,field:=range fields {field=strings.Trim(field,".,;:!?()[]{}\"");if len(field)>=4{out=append(out,field)}}
	return out
}
func nodeScore(name string,keywords []string) int {
	lower:=strings.ToLower(name);score:=0
	for _,keyword:=range keywords{if strings.Contains(lower,keyword){score+=5}}
	for _,core:=range []string{"loadimage","saveimage","checkpoint","ksampler","vae","cliptextencode","emptylatent","control","lineart","canny","inpaint","ipadapter"}{if strings.Contains(lower,core){score++}}
	return score
}
func stringScore(name string,keywords []string) int {
	lower:=strings.ToLower(name);score:=0
	for _,keyword:=range keywords{if strings.Contains(lower,keyword){score++}}
	return score
}
func mustJSONString(value any) string {data,_:=json.Marshal(value);return string(data)}

func flowPlanMap(plan FlowPlan) map[string]any {
	inputs:=make([]map[string]any,0,len(plan.Inputs))
	for _,input:=range plan.Inputs{inputs=append(inputs,map[string]any{"id":input.ID,"type":input.Type,"label":input.Label,"required":input.Required})}
	return map[string]any{"name":plan.Name,"description":plan.Description,"prompt":plan.Prompt,"negative_prompt":plan.NegativePrompt,"inputs":inputs,"parameters":plan.Parameters,"selected_model":plan.SelectedModel,"workflow":plan.Workflow,"reasoning":plan.Reasoning,"artist_steps":plan.ArtistSteps}
}
