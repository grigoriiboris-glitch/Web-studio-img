package assistant

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oleg3190/Web-studio-img/backend/internal/providers/comfyui"
 )

type ComfyTestCategory string

const (
	ComfyWorkflowCategory ComfyTestCategory = "workflow"
	ComfyRuntimeCategory ComfyTestCategory = "runtime"
	ComfyConnectionCategory ComfyTestCategory = "connection"
	ComfyContainerCategory ComfyTestCategory = "container"
	ComfyModelCategory ComfyTestCategory = "model"
	ComfyUnknownCategory ComfyTestCategory = "unknown"
 )

type ComfyTestError struct {
	Code string `json:"code"`
	Message string `json:"message"`
	Node string `json:"node,omitempty"`
	Input string `json:"input,omitempty"`
	Recoverable bool `json:"recoverable"`
}

type ComfyTestRepair struct {
	Available bool `json:"available"`
	Strategy string `json:"strategy"`
}

type ComfyTestResult struct {
	OK bool `json:"ok"`
	Category ComfyTestCategory `json:"category"`
	Errors []ComfyTestError `json:"errors"`
	Warnings []string `json:"warnings"`
	Repair ComfyTestRepair `json:"repair"`
	TestID string `json:"test_id,omitempty"`
	Duration int64 `json:"duration_ms,omitempty"`
}

func classifyComfyError(err error) (ComfyTestCategory, string, bool) {
	if err == nil { return ComfyUnknownCategory, "unknown", false }
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)
	switch {
	case errors.Is(err, comfyui.ErrProviderUnavailable):
		if strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "dial ") { return ComfyConnectionCategory, "comfy_connection", true }
		if strings.Contains(lower, "container") || strings.Contains(lower, "startup") || strings.Contains(lower, "import") || strings.Contains(lower, "dependency") { return ComfyContainerCategory, "comfy_container", true }
		return ComfyRuntimeCategory, "comfy_runtime", true
	case errors.Is(err, comfyui.ErrProviderCancelled):
		return ComfyRuntimeCategory, "comfy_cancelled", true
	case errors.Is(err, comfyui.ErrProviderInvalid):
		if strings.Contains(lower, "model") || strings.Contains(lower, "checkpoint") || strings.Contains(lower, "vae") || strings.Contains(lower, "lora") { return ComfyModelCategory, "comfy_model", true }
		if strings.Contains(lower, "node") || strings.Contains(lower, "input") || strings.Contains(lower, "workflow") || strings.Contains(lower, "connection") { return ComfyWorkflowCategory, "comfy_workflow", true }
		return ComfyRuntimeCategory, "comfy_execution", true
	default:
		return ComfyUnknownCategory, "comfy_unknown", false
	}
}

func comfyRepairFor(category ComfyTestCategory) ComfyTestRepair {
	switch category {
	case ComfyWorkflowCategory: return ComfyTestRepair{Available:true, Strategy:"ai_flow"}
	case ComfyConnectionCategory, ComfyContainerCategory, ComfyRuntimeCategory, ComfyModelCategory: return ComfyTestRepair{Available:true, Strategy:"pi_agent"}
	default: return ComfyTestRepair{Available:false, Strategy:"manual"}
	}
}

func comfyTestError(err error) ComfyTestError {
	category, code, recoverable := classifyComfyError(err)
	return ComfyTestError{Code:code, Message:fmt.Sprintf("%s: %s", category, strings.TrimSpace(err.Error())), Recoverable:recoverable}
}
