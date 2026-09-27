package similarity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/references"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Handler struct {
	refs       *references.Store
	assets     *assets.Store
	storage    storage.StorageProvider
	events     *events.Store
	provenance *provenance.Store
}

func NewHandler(refs *references.Store, assetsStore *assets.Store, storageProvider storage.StorageProvider, ev *events.Store, pv *provenance.Store) (*Handler, error) {
	if refs == nil || assetsStore == nil {
		return nil, errors.New("similarity handler requires stores")
	}
	return &Handler{refs: refs, assets: assetsStore, storage: storageProvider, events: ev, provenance: pv}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{project_id}/references/{reference_id}/influence", h.analyze)
}

func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		errJSON(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if h.storage == nil {
		errJSON(w, http.StatusServiceUnavailable, "storage_unavailable", "object storage is not configured")
		return
	}

	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return
	}
	referenceID, err := uuid.Parse(r.PathValue("reference_id"))
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid_reference_id", "invalid reference id")
		return
	}

	var in struct {
		TargetAssetID uuid.UUID `json:"target_asset_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil || in.TargetAssetID == uuid.Nil {
		errJSON(w, http.StatusBadRequest, "invalid_request", "target_asset_id is required")
		return
	}

	ref, err := h.refs.GetOwned(r.Context(), p.UserID, projectID, referenceID)
	if err != nil || ref.AssetID == nil {
		errJSON(w, http.StatusNotFound, "reference_asset_not_found", "reference must reference an asset")
		return
	}
	referenceAsset, err := h.assets.GetOwned(r.Context(), p.UserID, projectID, *ref.AssetID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "reference_asset_not_found", "reference asset not found")
		return
	}
	targetAsset, err := h.assets.GetOwned(r.Context(), p.UserID, projectID, in.TargetAssetID)
	if err != nil {
		errJSON(w, http.StatusNotFound, "target_asset_not_found", "target asset not found")
		return
	}

	referenceData, err := readObject(r.Context(), h.storage, referenceAsset.StorageKey)
	if err != nil {
		errJSON(w, http.StatusBadGateway, "reference_asset_read_failed", "could not read reference asset")
		return
	}
	targetData, err := readObject(r.Context(), h.storage, targetAsset.StorageKey)
	if err != nil {
		errJSON(w, http.StatusBadGateway, "target_asset_read_failed", "could not read target asset")
		return
	}

	scores, err := Scores(referenceData, targetData)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "influence_analysis_failed", "could not analyze image influence")
		return
	}
	influence := &references.Influence{
		Composition: scores["composition"],
		Semantic:    scores["semantic"],
		Color:       scores["color"],
		Style:       scores["style"],
		Material:    scores["material"],
		Geometry:    scores["geometry"],
	}
	if influence.Composition >= 0.8 {
		influence.Warning = "High composition similarity; review before use."
	}

	updated, err := h.refs.UpdateInfluence(r.Context(), p.UserID, projectID, referenceID, influence)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "influence_persist_failed", "could not persist influence analysis")
		return
	}

	payload := map[string]any{
		"target_asset_id": in.TargetAssetID,
		"influence":      updated.Influence,
	}
	if h.events != nil {
		_, _ = h.events.Append(r.Context(), p.UserID, projectID, "similarity.completed", "reference", referenceID, payload)
	}
	if h.provenance != nil {
		_, _ = h.provenance.Append(r.Context(), provenance.Event{
			UserID: p.UserID, ProjectID: projectID, EntityType: "reference", EntityID: referenceID,
			Action: "reference.influence_analyzed", Payload: payload,
		})
		if h.events != nil {
			_, _ = h.events.Append(r.Context(), p.UserID, projectID, "provenance.updated", "reference", referenceID, map[string]any{"action": "reference.influence_analyzed"})
		}
	}
	writeJSON(w, http.StatusOK, updated)
}

func readObject(ctx context.Context, s storage.StorageProvider, key string) ([]byte, error) {
	obj, _, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(io.LimitReader(obj, 10<<20+1))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg, "request_id": uuid.NewString()},
	})
}
