package comfyui

import "testing"

func TestMapWorkflowReplacesInputs(t *testing.T) {
	workflow := map[string]any{"1": map[string]any{"class_type":"Test","inputs":map[string]any{"prompt":"{{prompt}}","negative":"{{negative_prompt}}","seed":"{{seed}}","image":"{{input_image}}","mask":"{{mask_image}}","width":"{{width}}"}}}
	seed := int64(42)
	got, err := mapWorkflow(workflow, mapperValues{Prompt:"replace sky",NegativePrompt:"blur",Seed:&seed,InputImage:"source.png",MaskImage:"mask.png",Width:1024})
	if err != nil { t.Fatal(err) }
	inputs := got["1"].(map[string]any)["inputs"].(map[string]any)
	if inputs["prompt"] != "replace sky" || inputs["negative"] != "blur" || inputs["seed"] != int64(42) || inputs["image"] != "source.png" || inputs["mask"] != "mask.png" || inputs["width"] != int64(1024) { t.Fatalf("unexpected mapped workflow: %#v", inputs) }
}

func TestMapWorkflowDoesNotMutateTemplate(t *testing.T) {
	workflow := map[string]any{"1": map[string]any{"inputs":map[string]any{"prompt":"{{prompt}}"}}}
	_, err := mapWorkflow(workflow, mapperValues{Prompt:"new"})
	if err != nil { t.Fatal(err) }
	if workflow["1"].(map[string]any)["inputs"].(map[string]any)["prompt"] != "{{prompt}}" { t.Fatal("template was mutated") }
}
