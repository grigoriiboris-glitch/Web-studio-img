package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
)

const TaskType = "generation:execute"

type TaskPayload struct {
	UserID       uuid.UUID `json:"user_id"`
	GenerationID uuid.UUID `json:"generation_id"`
}

func NewTask(userID, generationID uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(TaskPayload{UserID: userID, GenerationID: generationID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskType, payload), nil
}

type Worker struct {
	Store    Store
	Provider Provider
	Assets   *assets.Processor
	Provenance *provenance.Store
	Events *events.Store
}

func (w *Worker) Handle(ctx context.Context, task *asynq.Task) error {
	if w.Assets != nil {
		_ = w.Assets.CleanupExpired(ctx, 50)
	}
	var payload TaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode generation task: %w", err)
	}
	item, err := w.Store.GetOwned(ctx, payload.UserID, payload.GenerationID)
	if err != nil {
		return err
	}
	if item.Status == StatusSucceeded || item.Status == StatusCancelled {
		return nil
	}
	now := time.Now()
	if err := w.Store.MarkRunning(ctx, payload.UserID, item.ID, now); err != nil && !errors.Is(err, ErrGenerationNotFound) {
		return err
	}
	if w.Events != nil {
		_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.started", "generation", item.ID, map[string]any{"status":"running"})
		_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.progress", "generation", item.ID, map[string]any{"status":"running","progress":0})
	}
	result, err := w.Provider.Generate(ctx, Request{
		ProjectID: item.ProjectID, IterationID: item.IterationID, Prompt: item.Prompt,
		NegativePrompt: item.NegativePrompt, Seed: item.Seed, AspectRatio: item.AspectRatio,
		Parameters: item.Parameters, IdempotencyKey: payload.GenerationID.String(),
	})
	if err != nil {
		code := "provider_error"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			code = "provider_timeout"
		}
		if !isRetryable(err) {
			_ = w.Store.MarkFailed(ctx, payload.UserID, item.ID, code, err.Error(), time.Now())
			if w.Events != nil {
			_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.progress", "generation", item.ID, map[string]any{"status":"failed","progress":100,"code":code})
			_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.failed", "generation", item.ID, map[string]any{"code":code,"message":err.Error()})
		}
			if w.Provenance != nil {
				_, _ = w.Provenance.Append(ctx, provenance.Event{UserID: payload.UserID, ProjectID: item.ProjectID, IterationID: item.IterationID, EntityType: "generation", EntityID: item.ID, Action: "generation_failed", Payload: map[string]any{"code": code, "message": err.Error()}, CreatedAt: time.Now()})
				if w.Events != nil { _, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "provenance.updated", "generation", item.ID, map[string]any{"action":"generation_failed"}) }
			}
			return nil
		}
		return err
	}
	for _, image := range result.Images {
		asset, err := w.Assets.Process(ctx, payload.UserID, item.ProjectID, item.ID, image.Data, image.ContentType)
		if err != nil {
			_ = w.Store.MarkFailed(ctx, payload.UserID, item.ID, "asset_error", err.Error(), time.Now())
			if w.Events != nil {
			_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.progress", "generation", item.ID, map[string]any{"status":"failed","progress":100,"code":"asset_error"})
			_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.failed", "generation", item.ID, map[string]any{"code":"asset_error","message":err.Error()})
		}
			if w.Provenance != nil {
				_, _ = w.Provenance.Append(ctx, provenance.Event{UserID: payload.UserID, ProjectID: item.ProjectID, IterationID: item.IterationID, EntityType: "generation", EntityID: item.ID, Action: "generation_failed", Payload: map[string]any{"code":"asset_error","message":err.Error()}, CreatedAt: time.Now()})
				if w.Events != nil { _, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "provenance.updated", "generation", item.ID, map[string]any{"action":"generation_failed"}) }
			}
			return nil
		}
		if w.Provenance != nil {
			_, _ = w.Provenance.Append(ctx, provenance.Event{UserID: payload.UserID, ProjectID: item.ProjectID, IterationID: item.IterationID, EntityType: "asset", EntityID: asset.ID, Action: "asset_created", Payload: map[string]any{"storage_key":asset.StorageKey,"preview_key":asset.PreviewKey,"thumbnail_key":asset.ThumbnailKey,"sha256":asset.Checksum,"mime_type":asset.MIMEType,"width":asset.Width,"height":asset.Height}, CreatedAt: time.Now()})
			if w.Events != nil { _, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "provenance.updated", "asset", asset.ID, map[string]any{"action":"asset_created"}) }
		}
	}
	var version *string
	if result.ModelVersion != "" {
		version = &result.ModelVersion
	}
	var jobID *string
	if result.ProviderJobID != "" {
		jobID = &result.ProviderJobID
	}
	if err := w.Store.MarkSucceeded(ctx, payload.UserID, item.ID, version, jobID, result.Cost, time.Now()); err != nil {
		return err
	}
	if w.Events != nil {
		_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.progress", "generation", item.ID, map[string]any{"status":"completed","progress":100})
		_, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "generation.completed", "generation", item.ID, map[string]any{"provider":item.Provider,"model":item.Model,"model_version":result.ModelVersion})
	}
	if w.Provenance != nil {
		_, _ = w.Provenance.Append(ctx, provenance.Event{UserID: payload.UserID, ProjectID: item.ProjectID, IterationID: item.IterationID, EntityType: "generation", EntityID: item.ID, Action: "generation_succeeded", Payload: map[string]any{"provider": item.Provider, "model": item.Model, "model_version": result.ModelVersion, "provider_job_id": result.ProviderJobID, "cost": result.Cost}, CreatedAt: time.Now()})
		if w.Events != nil { _, _ = w.Events.Append(ctx, payload.UserID, item.ProjectID, "provenance.updated", "generation", item.ID, map[string]any{"action":"generation_succeeded"}) }
	}
	return nil
}

func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var retryable interface{ Retryable() bool }
	return errors.As(err, &retryable) && retryable.Retryable()
}
