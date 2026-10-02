package assistant

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func flowFingerprint(steps []map[string]any) string {
	normalized := normalizeFlowSteps(steps)
	data, _ := json.Marshal(normalized)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func validateArtistFlowContract(expected []map[string]any, actual []map[string]any, workflow map[string]any) []string {
	expected = normalizeFlowSteps(expected)
	if len(expected) == 0 {
		return nil
	}
	errors := []string{}
	if len(actual) != len(expected) {
		errors = append(errors, fmt.Sprintf("planner returned %d artist steps, expected %d", len(actual), len(expected)))
	}
	for i, want := range expected {
		if i >= len(actual) {
			break
		}
		got := actual[i]
		if got["id"] != want["id"] {
			errors = append(errors, fmt.Sprintf("artist step %d is %v, expected %v", i, got["id"], want["id"]))
			continue
		}
		if got["enabled"] != want["enabled"] {
			errors = append(errors, fmt.Sprintf("artist step %s enabled state changed", want["id"]))
		}
		if got["order"] != want["order"] {
			errors = append(errors, fmt.Sprintf("artist step %s order changed", want["id"]))
		}
		evidence, _ := got["evidence"].([]any)
		if want["enabled"] == true && len(evidence) == 0 {
			errors = append(errors, fmt.Sprintf("enabled artist step %s has no execution evidence", want["id"]))
		}
		if want["enabled"] == false && len(evidence) > 0 {
			errors = append(errors, fmt.Sprintf("disabled artist step %s must not have execution evidence", want["id"]))
		}
		for _, raw := range evidence {
			item, ok := raw.(map[string]any)
			if !ok {
				errors = append(errors, fmt.Sprintf("artist step %s contains invalid evidence", want["id"]))
				continue
			}
			kind, _ := item["kind"].(string)
			value, _ := item["value"].(string)
			switch kind {
			case "asset":
				if !workflowContainsAsset(workflow, value) {
					errors = append(errors, fmt.Sprintf("artist step %s references asset %q which is not used by workflow", want["id"], value))
				}
			case "node":
				if _, ok := workflow[value]; !ok {
					errors = append(errors, fmt.Sprintf("artist step %s references missing workflow node %q", want["id"], value))
				}
			default:
				errors = append(errors, fmt.Sprintf("artist step %s has unknown evidence kind %q", want["id"], kind))
			}
		}
	}
	return errors
}

func workflowContainsAsset(workflow map[string]any, name string) bool {
	if name == "" {
		return false
	}
	_, ok := flowAssetRefs(workflow)[name]
	return ok
}
