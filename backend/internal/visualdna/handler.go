
package visualdna

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

const maxAnalysisBytes int64 = 10 << 20

var (
	errProjectNotFound  = errors.New("project not found")
	errSourceNotAllowed = errors.New("source asset not allowed")
	errNoSources        = errors.New("no analyzable source assets")
	errProfileNotFound  = errors.New("visual DNA profile not found")
)

type Handler struct {
	db      *sql.DB
	storage storage.StorageProvider
}

type sourceAsset struct {
	ID          uuid.UUID
	IterationID *uuid.UUID
	StorageKey  string
	Checksum    string
	MIMEType    string
	Size        int64
	Width       int
	Height      int
}

func NewHandler(db *sql.DB, provider storage.StorageProvider) (*Handler, error) {
	if db == nil {
		return nil, errors.New("visual DNA handler requires database")
	}
	return &Handler{db: db, storage: provider}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/visual-dna", h.list)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/visual-dna/sources", h.sources)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/visual-dna/analyze", h.analyze)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/visual-dna/profiles/{profile_id}", h.get)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/visual-dna/profiles/{profile_id}/recompute", h.recompute)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/visual-dna/profiles/{profile_id}/compare", h.compare)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/visual-dna/profiles/{profile_id}/suggestion", h.suggestion)
}

func currentUserID(r *http.Request) (uuid.UUID, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	return p.UserID, true
}

func parseProject(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		return uuid.Nil, errProjectNotFound
	}
	return id, nil
}

func (h *Handler) ownedProject(ctx context.Context, userID, projectID uuid.UUID) bool {
	var ok bool
	err := h.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')",
		projectID, userID).Scan(&ok)
	return err == nil && ok
}

func (h *Handler) sources(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	items, err := h.resolveSources(r.Context(), userID, projectID, nil, nil)
	if err != nil && !errors.Is(err, errNoSources) { writeError(w, http.StatusInternalServerError, "visual_dna_sources_failed", err.Error()); return }
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id": item.ID, "iteration_id": item.IterationID, "checksum": item.Checksum,
			"mime_type": item.MIMEType, "size": item.Size, "width": item.Width, "height": item.Height,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	rows, err := h.db.QueryContext(r.Context(),
		"SELECT id, version, algorithm_version, signals, summary, source_assets, source_iterations, uncertainty, created_at "+
			"FROM visual_dna_profiles WHERE user_id=$1 AND project_id=$2 ORDER BY version DESC LIMIT 100", userID, projectID)
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_list_failed", "could not list profiles"); return }
	defer func() { _ = rows.Close() }()
	out := make([]map[string]any, 0)
	for rows.Next() {
		item, err := scanProfile(rows)
		if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_list_failed", err.Error()); return }
		out = append(out, item)
	}
	if err := rows.Err(); err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_list_failed", err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"profiles": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	item, err := h.loadProfile(r.Context(), userID, projectID, r.PathValue("profile_id"))
	if errors.Is(err, errProfileNotFound) { writeError(w, http.StatusNotFound, "profile_not_found", "profile not found"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_get_failed", err.Error()); return }
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	input, err := decodeObject(r)
	if err != nil { writeError(w, http.StatusBadRequest, "invalid_request", "invalid visual DNA request"); return }
	assetIDs, err := parseUUIDList(rawStrings(input["asset_ids"]))
	if err != nil { writeError(w, http.StatusBadRequest, "invalid_asset_ids", "invalid asset id"); return }
	iterationIDs, err := parseUUIDList(rawStrings(input["iteration_ids"]))
	if err != nil { writeError(w, http.StatusBadRequest, "invalid_iteration_ids", "invalid iteration id"); return }
	sources, err := h.resolveSources(r.Context(), userID, projectID, assetIDs, iterationIDs)
	if errors.Is(err, errSourceNotAllowed) { writeError(w, http.StatusConflict, "source_asset_not_allowed", "one or more selected assets are unavailable or restricted"); return }
	if errors.Is(err, errNoSources) { writeError(w, http.StatusBadRequest, "no_analyzable_assets", "no analyzable assets were selected"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_sources_failed", err.Error()); return }
	if len(assetIDs) > 0 && len(sources) != len(uniqueUUIDs(assetIDs)) {
		writeError(w, http.StatusConflict, "source_asset_not_allowed", "one or more selected assets are unavailable or restricted"); return
	}
	profile, err := h.createProfile(r.Context(), userID, projectID, sources)
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_analysis_failed", err.Error()); return }
	writeJSON(w, http.StatusCreated, profile)
}

func (h *Handler) recompute(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	profile, err := h.loadProfile(r.Context(), userID, projectID, r.PathValue("profile_id"))
	if errors.Is(err, errProfileNotFound) { writeError(w, http.StatusNotFound, "profile_not_found", "profile not found"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_get_failed", err.Error()); return }
	raw, ok := profile["source_assets"].([]any)
	if !ok || len(raw) == 0 { writeError(w, http.StatusConflict, "profile_sources_missing", "profile has no source assets"); return }
	ids := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		if id, parseErr := uuid.Parse(fmt.Sprint(value)); parseErr == nil { ids = append(ids, id) }
	}
	sources, err := h.resolveSources(r.Context(), userID, projectID, ids, nil)
	if errors.Is(err, errSourceNotAllowed) || len(sources) != len(ids) {
		writeError(w, http.StatusConflict, "source_asset_not_allowed", "profile source assets are no longer available for analysis"); return
	}
	next, err := h.createProfile(r.Context(), userID, projectID, sources)
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_recompute_failed", err.Error()); return }
	writeJSON(w, http.StatusCreated, next)
}

func (h *Handler) compare(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	profile, err := h.loadProfile(r.Context(), userID, projectID, r.PathValue("profile_id"))
	if errors.Is(err, errProfileNotFound) { writeError(w, http.StatusNotFound, "profile_not_found", "profile not found"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_get_failed", err.Error()); return }
	targetID, err := uuid.Parse(r.URL.Query().Get("asset_id"))
	if err != nil { writeError(w, http.StatusBadRequest, "invalid_asset_id", "invalid target asset id"); return }
	targets, err := h.resolveSources(r.Context(), userID, projectID, []uuid.UUID{targetID}, nil)
	if errors.Is(err, errSourceNotAllowed) || len(targets) != 1 { writeError(w, http.StatusConflict, "source_asset_not_allowed", "target asset is unavailable or restricted"); return }
	target, err := h.analyzeAsset(r.Context(), targets[0])
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_compare_failed", err.Error()); return }

	sourceIDs, ok := profile["source_assets"].([]any)
	if !ok { writeError(w, http.StatusConflict, "profile_sources_missing", "profile has no source assets"); return }
	ids := make([]uuid.UUID, 0, len(sourceIDs))
	for _, value := range sourceIDs { if id, parseErr := uuid.Parse(fmt.Sprint(value)); parseErr == nil { ids = append(ids, id) } }
	sources, err := h.resolveSources(r.Context(), userID, projectID, ids, nil)
	if err != nil || len(sources) != len(ids) { writeError(w, http.StatusConflict, "source_asset_not_allowed", "profile sources are unavailable for comparison"); return }
	values := make([]AssetAnalysis, 0, len(sources))
	for _, source := range sources {
		value, err := h.analyzeAsset(r.Context(), source)
		if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_compare_failed", err.Error()); return }
		values = append(values, value)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile_id": profile["id"], "target": target.JSON(), "comparison": Compare(values, target),
		"algorithm_version": AlgorithmVersion,
		"uncertainty": "Comparison re-analyzes currently stored asset bytes; the stored profile is unchanged.",
	})
}

func (h *Handler) suggestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseProject(r)
	if err != nil || !h.ownedProject(r.Context(), userID, projectID) {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return
	}
	profile, err := h.loadProfile(r.Context(), userID, projectID, r.PathValue("profile_id"))
	if errors.Is(err, errProfileNotFound) { writeError(w, http.StatusNotFound, "profile_not_found", "profile not found"); return }
	if err != nil { writeError(w, http.StatusInternalServerError, "visual_dna_get_failed", err.Error()); return }
	signals, _ := profile["signals"].(map[string]any)
	suggestion := map[string]any{}
	for _, key := range []string{"aspect_ratio", "brightness", "contrast", "color_temperature", "saturation", "focal_distribution", "dominant_geometry", "material_texture", "lighting"} {
		if value, exists := signals[key]; exists { suggestion[key] = value }
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"automatic_prompt_mutation": false, "user_must_approve": true,
		"algorithm_version": profile["algorithm_version"], "suggestion": suggestion,
	})
}

func (h *Handler) resolveSources(ctx context.Context, userID, projectID uuid.UUID, assetIDs, iterationIDs []uuid.UUID) ([]sourceAsset, error) {
	args := []any{userID, projectID}
	conditions := []string{}
	next := 3
	if len(assetIDs) > 0 {
		placeholders := make([]string, 0, len(assetIDs))
		for _, id := range assetIDs { placeholders = append(placeholders, fmt.Sprintf("$%d", next)); args = append(args, id); next++ }
		conditions = append(conditions, "a.id IN ("+strings.Join(placeholders, ",")+")")
	}
	if len(iterationIDs) > 0 {
		placeholders := make([]string, 0, len(iterationIDs))
		for _, id := range iterationIDs { placeholders = append(placeholders, fmt.Sprintf("$%d", next)); args = append(args, id); next++ }
		conditions = append(conditions, "g.iteration_id IN ("+strings.Join(placeholders, ",")+")")
	}
	filter := ""
	if len(conditions) > 0 { filter = " AND ("+strings.Join(conditions, " OR ")+")" }
	rows, err := h.db.QueryContext(ctx,
		"SELECT a.id, g.iteration_id, a.storage_key, COALESCE(a.checksum, a.sha256, ''), a.mime_type, a.size, "+
			"COALESCE(a.width,0), COALESCE(a.height,0) "+
			"FROM assets a LEFT JOIN generations g ON g.id=a.generation_id "+
			"WHERE a.user_id=$1 AND a.project_id=$2 AND a.lifecycle_status='active' "+
			"AND NOT EXISTS (SELECT 1 FROM rights_registry rr WHERE rr.user_id=$1 AND rr.target_type='asset' AND rr.target_id=a.id AND rr.verification_state='restricted')"+
			filter+" ORDER BY a.created_at DESC, a.id DESC LIMIT 50", args...)
	if err != nil { return nil, fmt.Errorf("query visual DNA sources: %w", err) }
	defer func() { _ = rows.Close() }()
	out := make([]sourceAsset, 0)
	for rows.Next() {
		var item sourceAsset
		if err := rows.Scan(&item.ID, &item.IterationID, &item.StorageKey, &item.Checksum, &item.MIMEType, &item.Size, &item.Width, &item.Height); err != nil { return nil, err }
		out = append(out, item)
	}
	if err := rows.Err(); err != nil { return nil, err }
	if len(out) == 0 { return nil, errNoSources }
	if len(assetIDs) > 0 {
		found := map[uuid.UUID]bool{}
		for _, item := range out { found[item.ID] = true }
		for _, id := range uniqueUUIDs(assetIDs) { if !found[id] { return nil, errSourceNotAllowed } }
	}
	return out, nil
}

func (h *Handler) createProfile(ctx context.Context, userID, projectID uuid.UUID, sources []sourceAsset) (map[string]any, error) {
	if len(sources) == 0 { return nil, errNoSources }
	values := make([]AssetAnalysis, 0, len(sources))
	sourceAssets := make([]string, 0, len(sources))
	sourceIterations := map[uuid.UUID]bool{}
	for _, source := range sources {
		value, err := h.analyzeAsset(ctx, source)
		if err != nil { return nil, err }
		values = append(values, value)
		sourceAssets = append(sourceAssets, source.ID.String())
		if source.IterationID != nil { sourceIterations[*source.IterationID] = true }
	}
	signals, summary := BuildProfile(values)
	iterations := make([]string, 0, len(sourceIterations))
	for id := range sourceIterations { iterations = append(iterations, id.String()) }
	sort.Strings(iterations)
	rawSignals, err := json.Marshal(signals); if err != nil { return nil, err }
	rawSummary, err := json.Marshal(summary); if err != nil { return nil, err }
	rawAssets, err := json.Marshal(sourceAssets); if err != nil { return nil, err }
	rawIterations, err := json.Marshal(iterations); if err != nil { return nil, err }
	uncertainty := map[string]any{"summary": summary["uncertainty"], "algorithm_version": AlgorithmVersion}
	rawUncertainty, err := json.Marshal(uncertainty); if err != nil { return nil, err }
	var version int
	if err := h.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0)+1 FROM visual_dna_profiles WHERE user_id=$1 AND project_id=$2", userID, projectID).Scan(&version); err != nil { return nil, err }
	var id uuid.UUID
	err = h.db.QueryRowContext(ctx,
		"INSERT INTO visual_dna_profiles(user_id,project_id,name,version,algorithm_version,signals,summary,source_assets,source_iterations,uncertainty) "+
			"VALUES($1,$2,'Visual DNA',$3,$4,$5,$6,$7,$8,$9) RETURNING id",
		userID, projectID, version, AlgorithmVersion, rawSignals, rawSummary, rawAssets, rawIterations,
		rawUncertainty).Scan(&id)
	if err != nil { return nil, fmt.Errorf("create visual DNA profile: %w", err) }
	return h.loadProfileByID(ctx, userID, projectID, id)
}

func (h *Handler) analyzeAsset(ctx context.Context, item sourceAsset) (AssetAnalysis, error) {
	if h.storage == nil { return AssetAnalysis{}, errors.New("visual DNA storage provider is not configured") }
	reader, _, err := h.storage.Get(ctx, item.StorageKey)
	if err != nil { return AssetAnalysis{}, fmt.Errorf("load asset %s: %w", item.ID, err) }
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, maxAnalysisBytes+1))
	if err != nil { return AssetAnalysis{}, fmt.Errorf("read asset %s: %w", item.ID, err) }
	if int64(len(data)) > maxAnalysisBytes { return AssetAnalysis{}, fmt.Errorf("asset %s exceeds analysis size limit", item.ID) }
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return AssetAnalysis{}, fmt.Errorf("decode asset %s: %w", item.ID, err) }
	return Analyze(item.ID.String(), img)
}

func (h *Handler) loadProfile(ctx context.Context, userID, projectID uuid.UUID, value string) (map[string]any, error) {
	id, err := uuid.Parse(value)
	if err != nil { return nil, errProfileNotFound }
	return h.loadProfileByID(ctx, userID, projectID, id)
}

func (h *Handler) loadProfileByID(ctx context.Context, userID, projectID, id uuid.UUID) (map[string]any, error) {
	var rawSignals, rawSummary, rawAssets, rawIterations, rawUncertainty []byte
	var out = map[string]any{}
	var version int
	var algorithmVersion, name string
	var createdAt string
	err := h.db.QueryRowContext(ctx,
		"SELECT id, name, version, algorithm_version, signals, summary, source_assets, source_iterations, uncertainty, created_at "+
			"FROM visual_dna_profiles WHERE id=$1 AND user_id=$2 AND project_id=$3",
		id, userID, projectID).Scan(&id, &name, &version, &algorithmVersion, &rawSignals, &rawSummary, &rawAssets, &rawIterations, &rawUncertainty, &createdAt)
	if errors.Is(err, sql.ErrNoRows) { return nil, errProfileNotFound }
	if err != nil { return nil, err }
	var signals, summary, assets, iterations, uncertainty any
	_ = json.Unmarshal(rawSignals, &signals)
	_ = json.Unmarshal(rawSummary, &summary)
	_ = json.Unmarshal(rawAssets, &assets)
	_ = json.Unmarshal(rawIterations, &iterations)
	_ = json.Unmarshal(rawUncertainty, &uncertainty)
	out["id"]=id; out["name"]=name; out["version"]=version; out["algorithm_version"]=algorithmVersion
	out["signals"]=signals; out["summary"]=summary; out["source_assets"]=assets; out["source_iterations"]=iterations; out["uncertainty"]=uncertainty; out["created_at"]=createdAt
	return out, nil
}

func (h *Handler) scanProfile(rows *sql.Rows) (map[string]any, error) {
	var id uuid.UUID
	var version int
	var algorithm string
	var signals, summary, assets, iterations, uncertainty []byte
	var createdAt string
	if err := rows.Scan(&id, &version, &algorithm, &signals, &summary, &assets, &iterations, &uncertainty, &createdAt); err != nil { return nil, err }
	var s, sm, a, i, u any
	_ = json.Unmarshal(signals, &s); _ = json.Unmarshal(summary, &sm); _ = json.Unmarshal(assets, &a); _ = json.Unmarshal(iterations, &i); _ = json.Unmarshal(uncertainty, &u)
	return map[string]any{"id":id,"version":version,"algorithm_version":algorithm,"signals":s,"summary":sm,"source_assets":a,"source_iterations":i,"uncertainty":u,"created_at":createdAt}, nil
}

func decodeObject(r *http.Request) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(&object); err != nil { return nil, err }
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF { return nil, errors.New("request body must contain one JSON object") }
	return object, nil
}

func rawStrings(raw json.RawMessage) []string {
	if len(raw)==0 { return nil }
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(values))
	for _, value := range values { id, err := uuid.Parse(value); if err != nil { return nil, err }; out = append(out, id) }
	return out, nil
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	seen:=map[uuid.UUID]bool{}; out:=make([]uuid.UUID,0,len(values))
	for _, v:=range values { if !seen[v] { seen[v]=true; out=append(out,v) } }
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type","application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message,"request_id":uuid.NewString()}})
}
