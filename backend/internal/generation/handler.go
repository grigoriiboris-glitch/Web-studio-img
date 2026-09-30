package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
)

type Enqueuer interface {
	Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type ProjectPrivacyPolicy interface {
	AllowsProvider(context.Context, uuid.UUID, uuid.UUID, string) (bool, string, error)
}

type Handler struct {
	store Store
	queue Enqueuer
	provider Provider
	provenance *provenance.Store
	projectEvents *events.Store
	recipes RecipeResolver
	privacy ProjectPrivacyPolicy
}

func NewHandler(store Store, queue Enqueuer, provider Provider) (*Handler, error) {
	return NewHandlerWithDependencies(store, queue, provider, nil, nil)
}

func NewHandlerWithProvenance(store Store, queue Enqueuer, provider Provider, provenanceStore *provenance.Store) (*Handler, error) {
	return NewHandlerWithDependencies(store, queue, provider, provenanceStore, nil)
}

func NewHandlerWithDependencies(store Store, queue Enqueuer, provider Provider, provenanceStore *provenance.Store, eventStore *events.Store) (*Handler, error) {
	if store == nil || queue == nil || provider == nil {
		return nil, errors.New("generation handler requires store, queue and provider")
	}
	return &Handler{store: store, queue: queue, provider: provider, provenance: provenanceStore, projectEvents: eventStore}, nil
}

func NewHandlerWithRecipeResolver(store Store, queue Enqueuer, provider Provider, provenanceStore *provenance.Store, eventStore *events.Store, recipes RecipeResolver) (*Handler, error) {
	h, err := NewHandlerWithDependencies(store, queue, provider, provenanceStore, eventStore)
	if err != nil {
		return nil, err
	}
	h.recipes = recipes
	return h, nil
}

func (h *Handler) SetPrivacyPolicy(policy ProjectPrivacyPolicy) { h.privacy = policy }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{project_id}/generations", h.create)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/generations/{generation_id}", h.get)
	mux.HandleFunc("GET /api/v1/generations/{generation_id}", h.getGlobal)
	mux.HandleFunc("POST /api/v1/generations/{generation_id}/cancel", h.cancelGlobal)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/generations/{generation_id}/cancel", h.cancel)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/generations/{generation_id}/events", h.events)
}

type requestPayload struct {
	IterationID    *uuid.UUID     `json:"iteration_id,omitempty"`
	Prompt         string         `json:"prompt"`
	NegativePrompt string         `json:"negative_prompt,omitempty"`
	Seed           *int64         `json:"seed,omitempty"`
	AspectRatio    string         `json:"aspect_ratio,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
	ReferenceIDs   []uuid.UUID     `json:"reference_ids,omitempty"`
	RecipeID       *uuid.UUID      `json:"recipe_id,omitempty"`
	RecipeVersion  int             `json:"recipe_version,omitempty"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return
	}
	var input requestPayload
	if err := decode(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid generation payload")
		return
	}
	if h.privacy != nil {
		allowed, mode, policyErr := h.privacy.AllowsProvider(r.Context(), userID, projectID, h.provider.Name())
		if policyErr != nil {
			writeError(w, http.StatusInternalServerError, "privacy_policy_failed", "could not evaluate project privacy policy")
			return
		}
		if !allowed {
			if h.projectEvents != nil { _, _ = h.projectEvents.Append(r.Context(), userID, projectID, "generation.provider_rejected", "project", projectID, map[string]any{"provider": h.provider.Name(), "privacy_mode": mode}) }
			writeError(w, http.StatusForbidden, "provider_blocked_by_privacy_mode", "selected provider is blocked by project privacy mode")
			return
		}
	}
	req := Request{
		ProjectID: projectID, IterationID: input.IterationID, Prompt: input.Prompt,
		NegativePrompt: input.NegativePrompt, Seed: input.Seed, AspectRatio: input.AspectRatio,
		Parameters: input.Parameters, FinalParameters: input.Parameters, ReferenceIDs: input.ReferenceIDs,
		IdempotencyKey: r.Header.Get("Idempotency-Key"), RecipeID: input.RecipeID, RecipeVersion: input.RecipeVersion,
	}
	if input.RecipeID != nil {
		if h.recipes == nil {
			writeError(w, http.StatusServiceUnavailable, "recipe_unavailable", "recipe resolver is not configured")
			return
		}
		resolution, err := h.recipes.Resolve(r.Context(), userID, projectID, *input.RecipeID, input.RecipeVersion, input.Parameters)
		if err != nil {
			writeError(w, http.StatusConflict, "recipe_resolution_failed", err.Error())
			return
		}
		if !strings.EqualFold(resolution.Provider, h.provider.Name()) {
			writeError(w, http.StatusConflict, "recipe_provider_mismatch", "recipe provider does not match the active generation provider")
			return
		}
		req.RecipeVersion = resolution.Version
		req.Parameters = resolution.FinalParameters
		req.FinalParameters = resolution.FinalParameters
		req.ResolvedWorkflow = resolution.ResolvedWorkflow
		req.ResolvedWorkflowHash = resolution.ResolvedWorkflowHash
	}
	if req.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required")
		return
	}
	item, created, err := h.store.Create(r.Context(), userID, req, h.provider.Name(), h.provider.Model())
	if errors.Is(err, ErrInvalidGeneration) {
		writeError(w, http.StatusBadRequest, "invalid_generation", "generation payload is invalid")
		return
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used for another project")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation_create_failed", "could not create generation")
		return
	}
	if created {
		task, err := NewTask(userID, item.ID)
		if err != nil {
			_ = h.store.MarkFailed(r.Context(), userID, item.ID, "queue_encode_failed", err.Error(), time.Now())
			writeError(w, http.StatusInternalServerError, "queue_failed", "could not enqueue generation")
			return
		}
		if _, err := h.queue.Enqueue(r.Context(), task, asynq.MaxRetry(5), asynq.Timeout(6*time.Minute), asynq.Retention(24*time.Hour)); err != nil {
			_ = h.store.MarkFailed(r.Context(), userID, item.ID, "queue_failed", err.Error(), time.Now())
			writeError(w, http.StatusServiceUnavailable, "queue_failed", "could not enqueue generation")
			return
		}
	}
	if created && h.projectEvents != nil { _, _ = h.projectEvents.Append(r.Context(), userID, projectID, "generation.queued", "generation", item.ID, map[string]any{"provider": item.Provider, "model": item.Model}) }
	if created && h.provenance != nil {
		_, _ = h.provenance.Append(r.Context(), provenance.Event{UserID: userID, ProjectID: projectID, IterationID: req.IterationID, EntityType: "generation", EntityID: item.ID, Action: "generation_queued", Payload: map[string]any{"provider": item.Provider, "model": item.Model, "prompt": item.Prompt, "negative_prompt": item.NegativePrompt, "seed": item.Seed, "aspect_ratio": item.AspectRatio, "parameters": item.Parameters, "reference_ids": item.ReferenceIDs, "provider_deterministic": item.ProviderDeterministic, "determinism_note": item.DeterminismNote}, CreatedAt: time.Now()})
	}
	status := http.StatusAccepted
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, item)
}

func (h *Handler) getGlobal(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeError(w,401,"unauthorized","authentication required");return}
	id,err:=uuid.Parse(r.PathValue("generation_id"));if err!=nil{writeError(w,400,"invalid_generation_id","invalid generation id");return}
	item,err:=h.store.GetOwned(r.Context(),userID,id);if errors.Is(err,ErrGenerationNotFound){writeError(w,404,"generation_not_found","generation not found");return};if err!=nil{writeError(w,500,"generation_get_failed","could not load generation");return}
	writeJSON(w,200,item)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(r.PathValue("generation_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_generation_id", "invalid generation id")
		return
	}
	item, err := h.store.GetOwned(r.Context(), userID, id)
	if errors.Is(err, ErrGenerationNotFound) {
		writeError(w, http.StatusNotFound, "generation_not_found", "generation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation_get_failed", "could not load generation")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) cancelGlobal(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeError(w,401,"unauthorized","authentication required");return}
	id,err:=uuid.Parse(r.PathValue("generation_id"));if err!=nil{writeError(w,400,"invalid_generation_id","invalid generation id");return}
	item,err:=h.store.GetOwned(r.Context(),userID,id);if errors.Is(err,ErrGenerationNotFound){writeError(w,404,"generation_not_found","generation not found");return};if err!=nil{writeError(w,500,"generation_get_failed","could not load generation");return}
	if item.ProviderJobID!=nil{_ = h.provider.Cancel(r.Context(),*item.ProviderJobID)}
	if err:=h.store.MarkCancelled(r.Context(),userID,id,time.Now());err!=nil{writeError(w,409,"generation_not_cancellable","generation cannot be cancelled");return}
	if h.projectEvents!=nil{_,_=h.projectEvents.Append(r.Context(),userID,item.ProjectID,"generation.cancelled","generation",item.ID,nil)}
	item.Status=StatusCancelled;writeJSON(w,200,item)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(r.PathValue("generation_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_generation_id", "invalid generation id")
		return
	}
	item, err := h.store.GetOwned(r.Context(), userID, id)
	if errors.Is(err, ErrGenerationNotFound) {
		writeError(w, http.StatusNotFound, "generation_not_found", "generation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation_get_failed", "could not load generation")
		return
	}
	if item.ProviderJobID != nil {
		_ = h.provider.Cancel(r.Context(), *item.ProviderJobID)
	}
	if err := h.store.MarkCancelled(r.Context(), userID, id, time.Now()); err != nil {
		writeError(w, http.StatusConflict, "generation_not_cancellable", "generation cannot be cancelled")
		return
	}
	if h.projectEvents != nil { _, _ = h.projectEvents.Append(r.Context(), userID, item.ProjectID, "generation.cancelled", "generation", item.ID, nil) }
	item.Status = StatusCancelled
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(r.PathValue("generation_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_generation_id", "invalid generation id")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		item, err := h.store.GetOwned(r.Context(), userID, id)
		if errors.Is(err, ErrGenerationNotFound) {
			writeError(w, http.StatusNotFound, "generation_not_found", "generation not found")
			return
		}
		if err != nil {
			return
		}
		data, _ := json.Marshal(item)
		_, _ = fmt.Fprintf(w, "event: generation\ndata: %s\n\n", data)
		flusher.Flush()
		if item.Status == StatusSucceeded || item.Status == StatusFailed || item.Status == StatusCancelled {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func decode(r *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func currentUserID(r *http.Request) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": uuid.NewString()}})
}
