package comfyui

import "testing"

func TestValidateWorkflowAgainstSnapshotRejectsUnknownNodeAndModel(t *testing.T) {
	snapshot:=RuntimeCapabilitySnapshot{
		Nodes:map[string]NodeCapability{
			"LoadImage":{RequiredInputs:[]string{"image"}},
			"SaveImage":{RequiredInputs:[]string{"images"}},
		},
		Models:map[string][]string{"checkpoints":{"installed.safetensors"}},
	}
	result:=validateWorkflowAgainstSnapshot(map[string]any{
		"1":map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:sketch}}"}},
		"2":map[string]any{"class_type":"UnknownNode","inputs":map[string]any{}},
		"3":map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"missing.safetensors","ckpt_name":"missing.safetensors"}},
	},snapshot)
	if result.Compatible{t.Fatal("expected incompatible workflow")}
	if len(result.Errors)<2{t.Fatalf("expected node and model errors, got %#v",result.Errors)}
}

func TestValidateWorkflowAgainstSnapshotAcceptsLinks(t *testing.T) {
	snapshot:=RuntimeCapabilitySnapshot{
		Nodes:map[string]NodeCapability{
			"LoadImage":{RequiredInputs:[]string{"image"}},
			"SaveImage":{RequiredInputs:[]string{"images"}},
		},
		Models:map[string][]string{},
	}
	result:=validateWorkflowAgainstSnapshot(map[string]any{
		"1":map[string]any{"class_type":"LoadImage","inputs":map[string]any{"image":"{{asset:sketch}}"}},
		"2":map[string]any{"class_type":"SaveImage","inputs":map[string]any{"images":[]any{"1",0}}},
	},snapshot)
	if !result.Compatible{t.Fatalf("expected compatible workflow, got %#v",result.Errors)}
}
