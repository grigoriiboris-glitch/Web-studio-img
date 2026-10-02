package assistant

import "fmt"

func colorReferenceContractErrors(flowSteps []map[string]any, inputs []FlowInput, workflow map[string]any) []string {
	referenceEnabled := false
	for _, step := range flowSteps {
		if fmt.Sprint(step["id"]) == "reference" {
			if enabled, ok := step["enabled"].(bool); ok {
				referenceEnabled = enabled
			}
		}
	}
	if !referenceEnabled {
		return nil
	}

	declared := false
	required := false
	for _, input := range inputs {
		if input.ID == "color_reference" {
			declared = true
			required = input.Required
			break
		}
	}
	errors := []string{}
	if !declared {
		errors = append(errors, "enabled reference step requires a color_reference image input")
	} else if !required {
		errors = append(errors, "color_reference input must be required when the reference step is enabled")
	}

	refs := flowAssetRefs(workflow)
	if _, ok := refs["color_reference"]; !ok {
		errors = append(errors, "enabled reference step requires workflow evidence using {{asset:color_reference}}")
	}
	return errors
}
