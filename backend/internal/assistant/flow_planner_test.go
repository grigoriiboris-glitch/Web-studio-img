package assistant

import "testing"

func TestDecodeFlowPlan(t *testing.T) {
	plan,err:=decodeFlowPlan(`{"name":"Sketch Colorize","description":"test","prompt":"color the sketch","negative_prompt":"blurry","inputs":[{"id":"sketch","type":"image","label":"Sketch","required":true}],"parameters":{"width":1024,"height":1024},"selected_model":{"folder":"checkpoints","filename":"installed.safetensors"},"workflow":{"1":{"class_type":"LoadImage","inputs":{"image":"{{asset:sketch}}"}}},"reasoning":"preserve lineart"}`)
	if err!=nil{t.Fatalf("decode failed: %v",err)}
	if plan.Name!="Sketch Colorize"||len(plan.Workflow)!=1||len(plan.Inputs)!=1{t.Fatalf("unexpected plan: %#v",plan)}
	if plan.Inputs[0].ID!="sketch"{t.Fatalf("unexpected input: %#v",plan.Inputs[0])}
}
