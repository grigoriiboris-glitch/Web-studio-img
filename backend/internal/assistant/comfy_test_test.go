package assistant

import (
	"errors"
	"testing"

	"github.com/oleg3190/Web-studio-img/backend/internal/providers/comfyui"
)

func TestClassifyComfyError(t *testing.T) {
	tests := []struct {
		name string
		err error
		want ComfyTestCategory
	}{
		{"connection", fmtErr(comfyui.ErrProviderUnavailable, "connection refused"), ComfyConnectionCategory},
		{"model", fmtErr(comfyui.ErrProviderInvalid, "model checkpoints/foo.safetensors is missing"), ComfyModelCategory},
		{"workflow", fmtErr(comfyui.ErrProviderInvalid, "node KSampler contains unknown input"), ComfyWorkflowCategory},
		{"runtime", fmtErr(comfyui.ErrProviderInvalid, "workflow execution failed"), ComfyRuntimeCategory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _ := classifyComfyError(tt.err)
			if got != tt.want { t.Fatalf("category=%q want %q", got, tt.want) }
		})
	}
}

func TestComfyRepairStrategy(t *testing.T) {
	if got := comfyRepairFor(ComfyWorkflowCategory); got.Strategy != "ai_flow" || !got.Available { t.Fatalf("workflow strategy=%+v", got) }
	if got := comfyRepairFor(ComfyRuntimeCategory); got.Strategy != "pi_agent" || !got.Available { t.Fatalf("runtime strategy=%+v", got) }
	if got := comfyRepairFor(ComfyUnknownCategory); got.Strategy != "manual" || got.Available { t.Fatalf("unknown strategy=%+v", got) }
}

func fmtErr(base error, message string) error { return errors.Join(base, errors.New(message)) }

func TestClassifyComfyRuntimeDetails(t *testing.T) {
    tests := []struct {
        name string
        message string
        want string
    }{
        {"timeout", "execution timeout after 20ms", "comfy_timeout"},
        {"oom", "CUDA out of memory while allocating tensor", "comfy_oom"},
        {"cuda", "CUDA error: device-side assert", "comfy_cuda"},
        {"node", "node execution failed in KSampler", "comfy_node_execution"},
        {"dependency", "dependency module not found: xformers", "comfy_dependency"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, code, _ := classifyComfyError(fmtErr(comfyui.ErrProviderUnavailable, tt.message))
            if code != tt.want {
                t.Fatalf("code=%q want %q", code, tt.want)
            }
        })
    }
}
