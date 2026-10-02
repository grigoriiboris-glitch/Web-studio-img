package comfyui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
)

// TestResult is a real ComfyUI execution result. It deliberately does not
// create a generation record in Web Studio.
type TestResult struct {
	PromptID string
	Duration time.Duration
}

func (p *Provider) TestWorkflow(ctx context.Context, req generation.Request) (TestResult, error) {
	started := time.Now()
	if len(req.ResolvedWorkflow) == 0 {
		return TestResult{}, fmt.Errorf("%w: workflow is required", ErrProviderInvalid)
	}

	params := req.Parameters
	if params == nil {
		params = map[string]any{}
	}
	assets, err := p.uploadNamedAssets(ctx, params)
	if err != nil {
		return TestResult{}, err
	}

	inputImage, maskImage := "", ""
	if sourceKey := stringValue(params, "source_storage_key", ""); sourceKey != "" {
		inputImage, err = p.uploadAsset(ctx, "test_input_image", sourceKey)
		if err != nil {
			return TestResult{}, err
		}
	}
	if maskKey := stringValue(params, "mask_storage_key", ""); maskKey != "" {
		maskImage, err = p.uploadAsset(ctx, "test_mask_image", maskKey)
		if err != nil {
			return TestResult{}, err
		}
	}

	workflow, err := mapWorkflow(req.ResolvedWorkflow, mapperValues{
		Prompt: req.Prompt, NegativePrompt: req.NegativePrompt, Seed: req.Seed,
		AspectRatio: req.AspectRatio, InputImage: inputImage, MaskImage: maskImage,
		Width: intValue(params, "width", 0), Height: intValue(params, "height", 0),
		Operation: stringValue(params, "operation", "test"), Assets: assets,
	})
	if err != nil {
		return TestResult{}, fmt.Errorf("%w: %v", ErrProviderInvalid, err)
	}

	response, err := p.client.submit(ctx, workflow, "test-"+req.IdempotencyKey)
	if err != nil {
		return TestResult{}, err
	}

	pollCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	for {
		if err := pollCtx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				_ = p.client.interrupt(context.Background(), response.PromptID)
				return TestResult{}, fmt.Errorf("%w: %v", ErrProviderCancelled, err)
			}
			if errors.Is(err, context.DeadlineExceeded) {
				_ = p.client.interrupt(context.Background(), response.PromptID)
				return TestResult{}, fmt.Errorf("%w: execution timeout after %s", ErrProviderUnavailable, p.cfg.Timeout)
			}
			return TestResult{}, err
		}

		history, historyErr := p.client.history(pollCtx, response.PromptID)
		if historyErr == nil {
			if history.Status != nil && history.Status.StatusStr == "error" {
				return TestResult{}, fmt.Errorf("%w: workflow execution failed for prompt %s", ErrProviderInvalid, response.PromptID)
			}
			if (history.Status != nil && history.Status.Completed) || len(history.Outputs) > 0 {
				return TestResult{PromptID: response.PromptID, Duration: time.Since(started)}, nil
			}
		}

		timer := time.NewTimer(p.cfg.PollEvery)
		select {
		case <-pollCtx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
