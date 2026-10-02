package assistant

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "reflect"
    "sort"
)

// RepairInvariantInput describes creative/protected state that an AI repair is
// not allowed to mutate.
type RepairInvariantInput struct {
    FlowSteps             []map[string]any
    Inputs                any
    CreativeBrief         any
    ColorReferenceContract any
    ArtistIntent          any
}

// RepairInvariantResult is safe to persist in assistant action history.
type RepairInvariantResult struct {
    Valid             bool     `json:"valid"`
    Violations        []string `json:"violations,omitempty"`
    WorkflowBefore    string   `json:"workflow_fingerprint_before"`
    WorkflowAfter     string   `json:"workflow_fingerprint_after"`
    ProtectedFingerprintBefore string `json:"protected_fingerprint_before"`
    ProtectedFingerprintAfter  string `json:"protected_fingerprint_after"`
}

func checkRepairInvariants(input RepairInvariantInput, before, after map[string]any) RepairInvariantResult {
    result := RepairInvariantResult{
        Valid: true,
        WorkflowBefore: hashWorkflow(before),
        WorkflowAfter: hashWorkflow(after),
        ProtectedFingerprintBefore: hashProtectedRepairState(input),
    }

    beforeProtected := protectedRepairState(input)
    afterProtected := protectedRepairState(input)
    // The candidate may explicitly return protected fields. When present,
    // compare them to the authoritative request snapshot.
    if candidate, ok := after["flow_steps"]; ok {
        afterProtected["flow_steps"] = normalizeFlowSteps(candidate)
    }
    if candidate, ok := after["inputs"]; ok {
        afterProtected["inputs"] = candidate
    }
    if candidate, ok := after["creative_brief"]; ok {
        afterProtected["creative_brief"] = candidate
    }
    if candidate, ok := after["color_reference_contract"]; ok {
        afterProtected["color_reference_contract"] = candidate
    }
    if candidate, ok := after["artist_intent"]; ok {
        afterProtected["artist_intent"] = candidate
    }

    result.ProtectedFingerprintAfter = hashJSON(afterProtected)
    for _, key := range []string{"flow_steps", "inputs", "creative_brief", "color_reference_contract", "artist_intent"} {
        if !reflect.DeepEqual(beforeProtected[key], afterProtected[key]) {
            result.Valid = false
            result.Violations = append(result.Violations, fmt.Sprintf("protected field %s changed during AI repair", key))
        }
    }

    beforeAssets := sortedAssetRefs(before)
    afterAssets := sortedAssetRefs(after)
    if !reflect.DeepEqual(beforeAssets, afterAssets) {
        result.Valid = false
        result.Violations = append(result.Violations, "workflow asset inputs changed during AI repair")
    }
    return result
}

func protectedRepairState(input RepairInvariantInput) map[string]any {
    return map[string]any{
        "flow_steps": normalizeFlowSteps(input.FlowSteps),
        "inputs": input.Inputs,
        "creative_brief": input.CreativeBrief,
        "color_reference_contract": input.ColorReferenceContract,
        "artist_intent": input.ArtistIntent,
    }
}

func hashProtectedRepairState(input RepairInvariantInput) string {
    return hashJSON(protectedRepairState(input))
}

func hashJSON(value any) string {
    data, _ := json.Marshal(value)
    sum := sha256.Sum256(data)
    return fmt.Sprintf("%x", sum[:])
}

func sortedAssetRefs(workflow map[string]any) []string {
    refs := make([]string, 0, len(flowAssetRefs(workflow)))
    for ref := range flowAssetRefs(workflow) {
        refs = append(refs, ref)
    }
    sort.Strings(refs)
    return refs
}
