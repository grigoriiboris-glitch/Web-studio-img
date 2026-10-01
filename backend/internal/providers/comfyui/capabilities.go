
package comfyui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type NodeCapability struct {
	Category      string
	RequiredInputs []string
	OptionalInputs []string
	HiddenInputs  []string
	OutputTypes   []string
}

type RuntimeCapabilitySnapshot struct {
	RetrievedAt time.Time
	System      map[string]any
	Nodes       map[string]NodeCapability
	Models      map[string][]string
}

type WorkflowValidation struct {
	Compatible       bool
	Errors           []string
	Warnings         []string
	ReferencedNodes  []string
	ReferencedModels []string
}

func (p *Provider) CapabilitySnapshot(ctx context.Context) (RuntimeCapabilitySnapshot, error) {
	var system map[string]any
	if err := p.client.doJSON(ctx, "GET", "/system_stats", nil, &system); err != nil {
		return RuntimeCapabilitySnapshot{}, err
	}

	var rawNodes map[string]map[string]any
	if err := p.client.doJSON(ctx, "GET", "/object_info", nil, &rawNodes); err != nil {
		return RuntimeCapabilitySnapshot{}, err
	}

	nodes := make(map[string]NodeCapability, len(rawNodes))
	for name, raw := range rawNodes {
		nodes[name] = parseNodeCapability(raw)
	}

	models := map[string][]string{}
	var folders []string
	if err := p.client.doJSON(ctx, "GET", "/models", nil, &folders); err == nil {
		sort.Strings(folders)
		for _, folder := range folders {
			var names []string
			if err := p.client.doJSON(ctx, "GET", "/models/"+pathEscape(folder), nil, &names); err == nil {
				sort.Strings(names)
				models[folder] = names
			}
		}
	}

	return RuntimeCapabilitySnapshot{
		RetrievedAt: time.Now().UTC(),
		System: system,
		Nodes: nodes,
		Models: models,
	}, nil
}

func parseNodeCapability(raw map[string]any) NodeCapability {
	out := NodeCapability{}
	if v, ok := raw["category"].(string); ok {
		out.Category = v
	}
	out.RequiredInputs = keysFromSection(raw["input"], "required")
	out.OptionalInputs = keysFromSection(raw["input"], "optional")
	out.HiddenInputs = keysFromSection(raw["input"], "hidden")
	switch values := raw["output"].(type) {
	case []any:
		for _, value := range values {
			out.OutputTypes = append(out.OutputTypes, fmt.Sprint(value))
		}
	case []string:
		out.OutputTypes = append(out.OutputTypes, values...)
	}
	return out
}

func keysFromSection(input any, section string) []string {
	object, ok := input.(map[string]any)
	if !ok {
		return nil
	}
	sectionObject, ok := object[section].(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(sectionObject))
	for key := range sectionObject {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (p *Provider) ValidateWorkflow(ctx context.Context, workflow map[string]any) (WorkflowValidation, error) {
	snapshot, err := p.CapabilitySnapshot(ctx)
	if err != nil {
		return WorkflowValidation{}, err
	}
	return validateWorkflowAgainstSnapshot(workflow, snapshot), nil
}

func validateWorkflowAgainstSnapshot(workflow map[string]any, snapshot RuntimeCapabilitySnapshot) WorkflowValidation {
	result := WorkflowValidation{Compatible: true}
	if len(workflow) == 0 {
		return invalidValidation("workflow is empty")
	}

	nodeIDs := make(map[string]struct{}, len(workflow))
	for nodeID := range workflow {
		nodeIDs[nodeID] = struct{}{}
	}
	for nodeID, raw := range workflow {
		node, ok := raw.(map[string]any)
		if !ok {
			result.Errors = append(result.Errors, "node "+nodeID+" is not an object")
			continue
		}
		classType, _ := node["class_type"].(string)
		classType = strings.TrimSpace(classType)
		if classType == "" {
			result.Errors = append(result.Errors, "node "+nodeID+" has no class_type")
			continue
		}
		result.ReferencedNodes = append(result.ReferencedNodes, classType)
		info, ok := snapshot.Nodes[classType]
		if !ok {
			result.Errors = append(result.Errors, "node type "+classType+" is not installed in current ComfyUI")
			continue
		}
		inputs, _ := node["inputs"].(map[string]any)
		if inputs == nil {
			result.Errors = append(result.Errors, "node "+nodeID+" has no inputs object")
			continue
		}
		allowed := make(map[string]struct{}, len(info.RequiredInputs)+len(info.OptionalInputs)+len(info.HiddenInputs))
		for _, name := range info.RequiredInputs {
			allowed[name] = struct{}{}
		}
		for _, name := range info.OptionalInputs {
			allowed[name] = struct{}{}
		}
		for _, name := range info.HiddenInputs {
			allowed[name] = struct{}{}
		}
		for inputName, value := range inputs {
			if _, known := allowed[inputName]; !known {
				result.Errors = append(result.Errors, "node "+classType+" contains unknown input "+inputName+" for current ComfyUI")
			}
			validateLink(value, nodeID, nodeIDs, &result)
			if modelName, folder, ok := modelReference(inputName, value); ok {
				result.ReferencedModels = append(result.ReferencedModels, folder+"/"+modelName)
				if !modelInstalled(snapshot.Models, folder, modelName) {
					result.Errors = append(result.Errors, "model "+folder+"/"+modelName+" is not installed in current ComfyUI")
				}
			}
		}
		for _, required := range info.RequiredInputs {
			if _, present := inputs[required]; !present {
				result.Errors = append(result.Errors, "node "+nodeID+" is missing required input "+required)
			}
		}
	}
	sort.Strings(result.ReferencedNodes)
	result.ReferencedNodes = uniqueStrings(result.ReferencedNodes)
	sort.Strings(result.ReferencedModels)
	result.ReferencedModels = uniqueStrings(result.ReferencedModels)
	result.Compatible = len(result.Errors) == 0
	return result
}

func validateLink(value any, nodeID string, nodes map[string]struct{}, result *WorkflowValidation) {
	list, ok := value.([]any)
	if !ok || len(list) != 2 {
		return
	}
	ref := ""
	switch v := list[0].(type) {
	case string:
		ref = v
	case float64:
		ref = strconv.Itoa(int(v))
	case int:
		ref = strconv.Itoa(v)
	}
	if ref == "" {
		return
	}
	if _, exists := nodes[ref]; !exists {
		result.Errors = append(result.Errors, "node "+nodeID+" references missing node "+ref)
	}
}

func modelReference(inputName string, value any) (string, string, bool) {
	name := strings.TrimSpace(fmt.Sprint(value))
	if name == "" || strings.Contains(name, "{{") {
		return "", "", false
	}
	key := strings.ToLower(strings.ReplaceAll(inputName, "-", "_"))
	folder := ""
	switch key {
	case "ckpt_name", "checkpoint_name":
		folder = "checkpoints"
	case "unet_name", "unet":
		folder = "unet"
	case "diffusion_model", "diffusion_model_name":
		folder = "diffusion_models"
	case "vae_name":
		folder = "vae"
	case "clip_name":
		folder = "clip"
	case "lora_name":
		folder = "loras"
	case "control_net_name", "controlnet_name", "control_net":
		folder = "controlnet"
	case "upscale_model", "model_name" :
		if strings.HasSuffix(strings.ToLower(name), ".safetensors") || strings.HasSuffix(strings.ToLower(name), ".pth") {
			folder = "upscale_models"
		}
	case "clip_vision", "clip_vision_model":
		folder = "clip_vision"
	case "style_model_name":
		folder = "style_models"
	case "ipadapter_file":
		folder = "ipadapter"
	case "sam_model_name":
		folder = "sams"
	}
	if folder == "" {
		return "", "", false
	}
	return name, folder, true
}

func modelInstalled(models map[string][]string, folder, name string) bool {
	for _, candidate := range models[folder] {
		if candidate == name {
			return true
		}
	}
	for _, names := range models {
		for _, candidate := range names {
			if candidate == name {
				return true
			}
		}
	}
	return false
}

func invalidValidation(message string) WorkflowValidation {
	return WorkflowValidation{Compatible: false, Errors: []string{message}}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func pathEscape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "/", "%2F"), "?", "%3F")
}
