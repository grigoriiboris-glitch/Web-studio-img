package similarity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/references"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Handler struct {
	db         *sql.DB
	refs       *references.Store
	assets     *assets.Store
	storage    storage.StorageProvider
	events     *events.Store
	provenance *provenance.Store
}

type Check struct {
	ID                 uuid.UUID              `json:"id"`
	ProjectID          uuid.UUID              `json:"project_id"`
	UserID             uuid.UUID              `json:"user_id"`
	TargetAssetID      uuid.UUID              `json:"target_asset_id"`
	VisualScore        float64                `json:"visual_score"`
	CompositionScore   float64                `json:"composition_score"`
	SemanticScore      float64                `json:"semantic_score"`
	StyleScore         float64                `json:"style_score"`
	SearchScope        string                 `json:"search_scope"`
	Sources            []string               `json:"sources"`
	UnavailableSources []string               `json:"unavailable_sources"`
	Algorithm          string                 `json:"algorithm"`
	AlgorithmVersion   string                 `json:"algorithm_version"`\n\tIdempotencyKey     string                 `json:"-"`
	Metadata           map[string]any         `json:"metadata"`
	CreatedAt          time.Time              `json:"created_at"`
}

func NewHandler(refs *references.Store, assetsStore *assets.Store, storageProvider storage.StorageProvider, ev *events.Store, pv *provenance.Store) (*Handler, error) {
	return NewHandlerWithDB(nil, refs, assetsStore, storageProvider, ev, pv)
}
func NewHandlerWithDB(db *sql.DB, refs *references.Store, assetsStore *assets.Store, storageProvider storage.StorageProvider, ev *events.Store, pv *provenance.Store) (*Handler, error) {
	if refs == nil || assetsStore == nil {
		return nil, errors.New("similarity handler requires stores")
	}
	return &Handler{db: db, refs: refs, assets: assetsStore, storage: storageProvider, events: ev, provenance: pv}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{project_id}/similarity-checks", h.createCheck)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/similarity-checks/{check_id}", h.getCheck)
	mux.HandleFunc("GET /api/v1/similarity-checks/{check_id}", h.getCheck)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/references/{reference_id}/influence", h.analyzeInfluence)
}

func (h *Handler) createCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { errJSON(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	if h.db == nil || h.storage == nil { errJSON(w, http.StatusServiceUnavailable, "similarity_unavailable", "similarity resource storage is not configured"); return }
	if strings.TrimSpace(r.Header.Get("Idempotency-Key")) == "" { errJSON(w, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required"); return }
	projectID, err := parseUUID(w, r, "project_id")
	if err != nil { return }
	var in struct {
		TargetAssetID uuid.UUID `json:"target_asset_id"`
		ReferenceIDs  []uuid.UUID `json:"reference_ids"`
		SearchScope   string     `json:"search_scope"`
	}
	if err := decodeJSON(r, &in); err != nil || in.TargetAssetID == uuid.Nil {
		errJSON(w, http.StatusBadRequest, "invalid_request", "target_asset_id is required"); return
	}
	target, err := h.assets.GetOwned(r.Context(), p.UserID, projectID, in.TargetAssetID)
	if err != nil { errJSON(w, http.StatusNotFound, "target_asset_not_found", "target asset not found"); return }
	refs, err := h.refs.List(r.Context(), p.UserID, projectID)
	if err != nil { errJSON(w, http.StatusInternalServerError, "similarity_sources_failed", "could not load references"); return }
	selected := refs
	if len(in.ReferenceIDs) > 0 {
		selected = nil
		lookup := make(map[uuid.UUID]bool, len(in.ReferenceIDs))
		for _, id := range in.ReferenceIDs { lookup[id] = true }
		for _, item := range refs { if lookup[item.ID] { selected = append(selected, item) } }
	}
	if strings.TrimSpace(in.SearchScope) == "" { in.SearchScope = "project_references" }
	var maxVisual, maxComposition, maxSemantic, maxStyle float64
	sources := make([]string, 0, len(selected))
	unavailable := make([]string, 0)
	perReference := make([]map[string]any, 0, len(selected))
	targetData, err := readObject(r.Context(), h.storage, target.StorageKey)
	if err != nil { errJSON(w, http.StatusBadGateway, "target_asset_read_failed", "could not read target asset"); return }
	for _, ref := range selected {
		sources = append(sources, ref.ID.String())
		if ref.AssetID == nil { unavailable = append(unavailable, ref.ID.String()); continue }
		refAsset, err := h.assets.GetOwned(r.Context(), p.UserID, projectID, *ref.AssetID)
		if err != nil { unavailable = append(unavailable, ref.ID.String()); continue }
		refData, err := readObject(r.Context(), h.storage, refAsset.StorageKey)
		if err != nil { unavailable = append(unavailable, ref.ID.String()); continue }
		result, err := Analyze(refData, targetData)
		if err != nil { unavailable = append(unavailable, ref.ID.String()); continue }
		maxVisual = math.Max(maxVisual, result.Visual)
		maxComposition = math.Max(maxComposition, result.Composition)
		maxSemantic = math.Max(maxSemantic, result.Semantic)
		maxStyle = math.Max(maxStyle, result.Style)
		perReference = append(perReference, map[string]any{
			"reference_id": ref.ID,
			"visual_score": result.Visual, "composition_score": result.Composition,
			"semantic_score": result.Semantic, "style_score": result.Style,
			"perceptual_hash_score": result.PHashScore,
			"embedding_score": result.EmbeddingScore,
			"metadata_match": result.MetadataMatch,
			"composition_descriptor_score": result.CompositionDescriptorScore,
		})
	}
	metadata := map[string]any{
		"results": perReference,
		"mvp_search": map[string]bool{"perceptual_hash": true, "image_embeddings": true, "metadata_matching": true, "composition_descriptors": true},
		"legal_note": "Similarity analysis is not a legal conclusion and does not establish that all internet sources were checked.",
	}
	rawSources, _ := json.Marshal(sources)
	rawUnavailable, _ := json.Marshal(unavailable)
	rawMetadata, _ := json.Marshal(metadata)
	var c Check
	var created time.Time
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO similarity_checks(project_id,user_id,target_asset_id,visual_score,composition_score,semantic_score,style_score,search_scope,sources,unavailable_sources,algorithm,algorithm_version,metadata,idempotency_key)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (user_id,idempotency_key) DO NOTHING
		RETURNING id,project_id,user_id,target_asset_id,visual_score,composition_score,semantic_score,style_score,search_scope,sources,unavailable_sources,algorithm,algorithm_version,metadata,created_at
	`, projectID,p.UserID,target.ID,maxVisual,maxComposition,maxSemantic,maxStyle,in.SearchScope,rawSources,rawUnavailable,"deterministic-image-descriptors",AlgorithmVersion,rawMetadata,idempotencyKey).Scan(
		&c.ID,&c.ProjectID,&c.UserID,&c.TargetAssetID,&c.VisualScore,&c.CompositionScore,&c.SemanticScore,&c.StyleScore,&c.SearchScope,&rawSources,&rawUnavailable,&c.Algorithm,&c.AlgorithmVersion,&rawMetadata,&created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		var existingTarget uuid.UUID
		err = h.db.QueryRowContext(r.Context(), `SELECT id,target_asset_id FROM similarity_checks WHERE user_id=$1 AND idempotency_key=$2`, p.UserID, idempotencyKey).Scan(&c.ID, &existingTarget)
		if err != nil { errJSON(w, 500, "similarity_persist_failed", "could not load idempotent similarity check"); return }
		if existingTarget != target.ID { errJSON(w, 409, "idempotency_conflict", "idempotency key was already used for another target asset"); return }
		writeJSON(w, http.StatusOK, mustLoadCheck(h.db, r.Context(), p.UserID, c.ID)); return
	}
	if err != nil { errJSON(w, http.StatusInternalServerError, "similarity_persist_failed", "could not persist similarity check"); return }
	c.CreatedAt = created
	_ = json.Unmarshal(rawSources, &c.Sources); _ = json.Unmarshal(rawUnavailable, &c.UnavailableSources); _ = json.Unmarshal(rawMetadata, &c.Metadata)
	payload := map[string]any{"check_id": c.ID, "target_asset_id": c.TargetAssetID, "visual_score": c.VisualScore, "composition_score": c.CompositionScore, "semantic_score": c.SemanticScore, "style_score": c.StyleScore, "search_scope": c.SearchScope}
	if h.events != nil { _, _ = h.events.Append(r.Context(), p.UserID, projectID, "similarity.completed", "similarity_check", c.ID, payload) }
	if h.provenance != nil { _, _ = h.provenance.Append(r.Context(), provenance.Event{UserID:p.UserID,ProjectID:projectID,EntityType:"similarity_check",EntityID:c.ID,Action:"similarity.completed",Payload:payload}) }
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) getCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { errJSON(w, 401, "unauthorized", "authentication required"); return }
	if h.db == nil { errJSON(w, 503, "similarity_unavailable", "similarity resource storage is not configured"); return }
	id, err := uuid.Parse(r.PathValue("check_id")); if err != nil { errJSON(w,400,"invalid_check_id","invalid similarity check id"); return }
	var c Check; var sources, unavailable, metadata []byte
	err = h.db.QueryRowContext(r.Context(), `
		SELECT s.id,s.project_id,s.user_id,s.target_asset_id,s.visual_score,s.composition_score,s.semantic_score,s.style_score,s.search_scope,s.sources,s.unavailable_sources,s.algorithm,s.algorithm_version,s.metadata,s.created_at
		FROM similarity_checks s JOIN projects p ON p.id=s.project_id
		WHERE s.id=$1 AND s.user_id=$2 AND p.user_id=$2 AND p.status <> 'deleted'
	`, id,p.UserID).Scan(&c.ID,&c.ProjectID,&c.UserID,&c.TargetAssetID,&c.VisualScore,&c.CompositionScore,&c.SemanticScore,&c.StyleScore,&c.SearchScope,&sources,&unavailable,&c.Algorithm,&c.AlgorithmVersion,&metadata,&c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { errJSON(w,404,"similarity_check_not_found","similarity check not found"); return }
	if err != nil { errJSON(w,500,"similarity_check_failed","could not load similarity check"); return }
	_ = json.Unmarshal(sources,&c.Sources); _ = json.Unmarshal(unavailable,&c.UnavailableSources); _ = json.Unmarshal(metadata,&c.Metadata)
	writeJSON(w,200,c)
}

func (h *Handler) analyzeInfluence(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { errJSON(w,401,"unauthorized","authentication required"); return }
	if h.storage == nil { errJSON(w,503,"storage_unavailable","object storage is not configured"); return }
	projectID, err := uuid.Parse(r.PathValue("project_id")); if err != nil { errJSON(w,400,"invalid_project_id","invalid project id"); return }
	referenceID, err := uuid.Parse(r.PathValue("reference_id")); if err != nil { errJSON(w,400,"invalid_reference_id","invalid reference id"); return }
	var in struct{ TargetAssetID uuid.UUID `json:"target_asset_id"` }
	if err := decodeJSON(r,&in) || in.TargetAssetID == uuid.Nil { errJSON(w,400,"invalid_request","target_asset_id is required"); return }
	ref, err := h.refs.GetOwned(r.Context(),p.UserID,projectID,referenceID)
	if err != nil || ref.AssetID == nil { errJSON(w,404,"reference_asset_not_found","reference must reference an asset"); return }
	referenceAsset, err := h.assets.GetOwned(r.Context(),p.UserID,projectID,*ref.AssetID); if err != nil { errJSON(w,404,"reference_asset_not_found","reference asset not found"); return }
	targetAsset, err := h.assets.GetOwned(r.Context(),p.UserID,projectID,in.TargetAssetID); if err != nil { errJSON(w,404,"target_asset_not_found","target asset not found"); return }
	referenceData, err := readObject(r.Context(),h.storage,referenceAsset.StorageKey); if err != nil { errJSON(w,502,"reference_asset_read_failed","could not read reference asset"); return }
	targetData, err := readObject(r.Context(),h.storage,targetAsset.StorageKey); if err != nil { errJSON(w,502,"target_asset_read_failed","could not read target asset"); return }
	scores, err := Scores(referenceData,targetData); if err != nil { errJSON(w,400,"influence_analysis_failed","could not analyze image influence"); return }
	influence := &references.Influence{Composition:scores["composition"],Semantic:scores["semantic"],Color:scores["color"],Style:scores["style"],Material:scores["material"],Geometry:scores["geometry"]}
	updated, err := h.refs.UpdateInfluence(r.Context(),p.UserID,projectID,referenceID,influence); if err != nil { errJSON(w,500,"influence_persist_failed","could not persist influence analysis"); return }
	payload:=map[string]any{"target_asset_id":in.TargetAssetID,"influence":updated.Influence}
	if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,projectID,"similarity.completed","reference",referenceID,payload)}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:p.UserID,ProjectID:projectID,EntityType:"reference",EntityID:referenceID,Action:"reference.influence_analyzed",Payload:payload})}
	writeJSON(w,200,updated)
}

func parseUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name)); if err != nil { errJSON(w,400,"invalid_"+name,"invalid "+name); return uuid.Nil,err }
	return id,nil
}
func decodeJSON(r *http.Request,v any) error { d:=json.NewDecoder(io.LimitReader(r.Body,1<<20)); d.DisallowUnknownFields(); if err:=d.Decode(v);err!=nil{return err}; var extra any; if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")}; return nil }
func readObject(ctx context.Context,s storage.StorageProvider,key string)([]byte,error){obj,_,err:=s.Get(ctx,key);if err!=nil{return nil,err};defer obj.Close();return io.ReadAll(io.LimitReader(obj,10<<20+1))}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func errJSON(w http.ResponseWriter,status int,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}

func mustLoadCheck(db *sql.DB, ctx context.Context, userID, id uuid.UUID) Check {
	var c Check
	var sources, unavailable, metadata []byte
	_ = db.QueryRowContext(ctx, `SELECT id,project_id,user_id,target_asset_id,visual_score,composition_score,semantic_score,style_score,search_scope,sources,unavailable_sources,algorithm,algorithm_version,metadata,created_at FROM similarity_checks WHERE id=$1 AND user_id=$2`, id, userID).
		Scan(&c.ID,&c.ProjectID,&c.UserID,&c.TargetAssetID,&c.VisualScore,&c.CompositionScore,&c.SemanticScore,&c.StyleScore,&c.SearchScope,&sources,&unavailable,&c.Algorithm,&c.AlgorithmVersion,&metadata,&c.CreatedAt)
	_ = json.Unmarshal(sources,&c.Sources); _ = json.Unmarshal(unavailable,&c.UnavailableSources); _ = json.Unmarshal(metadata,&c.Metadata)
	return c
}
