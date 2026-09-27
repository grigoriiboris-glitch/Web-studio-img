package composition

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Spec struct {
	ID uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	IterationID uuid.UUID `json:"iteration_id"`
	UserID uuid.UUID `json:"user_id"`
	FocalPoints []any `json:"focal_points"`
	BoundingBoxes []any `json:"bounding_boxes"`
	RelativePositions map[string]any `json:"relative_positions"`
	Horizon *float64 `json:"horizon,omitempty"`
	CameraElevation *float64 `json:"camera_elevation,omitempty"`
	Perspective string `json:"perspective,omitempty"`
	Hierarchy []any `json:"hierarchy"`
	NegativeSpace map[string]any `json:"negative_space"`
	DominantGeometry map[string]any `json:"dominant_geometry"`
	ObjectScale map[string]any `json:"object_scale"`
	LightDirection map[string]any `json:"light_direction"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Mutation struct {
	ID uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	UserID uuid.UUID `json:"user_id"`
	CompositionSpecID *uuid.UUID `json:"composition_spec_id,omitempty"`
	SourceSimilarityCheckID *uuid.UUID `json:"source_similarity_check_id,omitempty"`
	Suggestions []string `json:"suggestions"`
	Status string `json:"status"`
	AcceptedIterationID *uuid.UUID `json:"accepted_iteration_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Handler struct {
	db *sql.DB
	events *events.Store
	provenance *provenance.Store
	actions *humanactions.Store
}

func NewHandler(db *sql.DB, ev *events.Store, pv *provenance.Store, actions *humanactions.Store) (*Handler, error) {
	if db == nil {
		return nil, errors.New("composition handler requires database")
	}
	return &Handler{db: db, events: ev, provenance: pv, actions: actions}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/iterations/{iteration_id}/composition", h.getSpec)
	mux.HandleFunc("PUT /api/v1/projects/{project_id}/iterations/{iteration_id}/composition", h.putSpec)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/composition-mutation-suggestions", h.suggest)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/composition-mutation-suggestions/{mutation_id}/accept", h.accept)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/composition-mutation-suggestions/{mutation_id}/reject", h.reject)
}

func (h *Handler) putSpec(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.PrincipalFromContext(r.Context())
	if !ok { writeErr(w, 401, "unauthorized", "authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id")); if err != nil { writeErr(w, 400, "invalid_project_id", "invalid project id"); return }
	iid, err := uuid.Parse(r.PathValue("iteration_id")); if err != nil { writeErr(w, 400, "invalid_iteration_id", "invalid iteration id"); return }
	var in struct {
		FocalPoints []any `json:"focal_points"`
		BoundingBoxes []any `json:"bounding_boxes"`
		RelativePositions map[string]any `json:"relative_positions"`
		Horizon *float64 `json:"horizon"`
		CameraElevation *float64 `json:"camera_elevation"`
		Perspective string `json:"perspective"`
		Hierarchy []any `json:"hierarchy"`
		NegativeSpace map[string]any `json:"negative_space"`
		DominantGeometry map[string]any `json:"dominant_geometry"`
		ObjectScale map[string]any `json:"object_scale"`
		LightDirection map[string]any `json:"light_direction"`
	}
	if err := decode(r, &in); err != nil { writeErr(w, 400, "invalid_request", "invalid composition payload"); return }
	if in.Horizon != nil && (*in.Horizon < 0 || *in.Horizon > 1) { writeErr(w, 400, "invalid_horizon", "horizon must be between 0 and 1"); return }
	var exists bool
	if err := h.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM iterations i JOIN projects p ON p.id=i.project_id WHERE i.id=$1 AND i.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted')`, iid, pid, u.UserID).Scan(&exists); err != nil || !exists {
		writeErr(w, 404, "iteration_not_found", "iteration not found"); return
	}
	raw := func(v any) []byte { b, _ := json.Marshal(v); if len(b) == 0 { return []byte("{}") }; return b }
	var s Spec
	var fp, bb, rp, hi, ns, dg, os, ld []byte
	err = h.db.QueryRowContext(r.Context(), `INSERT INTO composition_specs(project_id,iteration_id,user_id,focal_points,bounding_boxes,relative_positions,horizon,camera_elevation,perspective,hierarchy,negative_space,dominant_geometry,object_scale,light_direction)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT(iteration_id) DO UPDATE SET focal_points=excluded.focal_points,bounding_boxes=excluded.bounding_boxes,relative_positions=excluded.relative_positions,horizon=excluded.horizon,camera_elevation=excluded.camera_elevation,perspective=excluded.perspective,hierarchy=excluded.hierarchy,negative_space=excluded.negative_space,dominant_geometry=excluded.dominant_geometry,object_scale=excluded.object_scale,light_direction=excluded.light_direction,updated_at=now()
		RETURNING id,project_id,iteration_id,user_id,focal_points,bounding_boxes,relative_positions,horizon,camera_elevation,perspective,hierarchy,negative_space,dominant_geometry,object_scale,light_direction,created_at,updated_at`,
		pid, iid, u.UserID, raw(in.FocalPoints), raw(in.BoundingBoxes), raw(in.RelativePositions), in.Horizon, in.CameraElevation, strings.TrimSpace(in.Perspective), raw(in.Hierarchy), raw(in.NegativeSpace), raw(in.DominantGeometry), raw(in.ObjectScale), raw(in.LightDirection),
	).Scan(&s.ID, &s.ProjectID, &s.IterationID, &s.UserID, &fp, &bb, &rp, &s.Horizon, &s.CameraElevation, &s.Perspective, &hi, &ns, &dg, &os, &ld, &s.CreatedAt, &s.UpdatedAt)
	if err != nil { writeErr(w, 500, "composition_save_failed", "could not persist composition"); return }
	_ = json.Unmarshal(fp, &s.FocalPoints); _ = json.Unmarshal(bb, &s.BoundingBoxes); _ = json.Unmarshal(rp, &s.RelativePositions)
	_ = json.Unmarshal(hi, &s.Hierarchy); _ = json.Unmarshal(ns, &s.NegativeSpace); _ = json.Unmarshal(dg, &s.DominantGeometry); _ = json.Unmarshal(os, &s.ObjectScale); _ = json.Unmarshal(ld, &s.LightDirection)
	payload := map[string]any{"iteration_id": iid, "composition_spec_id": s.ID}
	if h.events != nil { _, _ = h.events.Append(r.Context(), u.UserID, pid, "iteration.updated", "iteration", iid, payload) }
	if h.provenance != nil { _, _ = h.provenance.Append(r.Context(), provenance.Event{UserID:u.UserID,ProjectID:pid,IterationID:&iid,EntityType:"composition",EntityID:s.ID,Action:"composition.updated",Payload:payload}) }
	if h.actions != nil { _, _ = h.actions.Create(r.Context(), u.UserID, pid, humanactions.Request{IterationID:&iid,ActionType:"COMPOSITION_CHANGED",Payload:payload,NewState:payload}) }
	writeJSON(w, 200, s)
}

func (h *Handler) getSpec(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.PrincipalFromContext(r.Context()); if !ok { writeErr(w,401,"unauthorized","authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id")); if err != nil { writeErr(w,400,"invalid_project_id","invalid project id"); return }
	iid, err := uuid.Parse(r.PathValue("iteration_id")); if err != nil { writeErr(w,400,"invalid_iteration_id","invalid iteration id"); return }
	var s Spec; var fp,bb,rp,hi,ns,dg,os,ld []byte
	err = h.db.QueryRowContext(r.Context(), `SELECT c.id,c.project_id,c.iteration_id,c.user_id,c.focal_points,c.bounding_boxes,c.relative_positions,c.horizon,c.camera_elevation,c.perspective,c.hierarchy,c.negative_space,c.dominant_geometry,c.object_scale,c.light_direction,c.created_at,c.updated_at
		FROM composition_specs c JOIN projects p ON p.id=c.project_id
		WHERE c.iteration_id=$1 AND c.project_id=$2 AND p.user_id=$3`, iid,pid,u.UserID).Scan(&s.ID,&s.ProjectID,&s.IterationID,&s.UserID,&fp,&bb,&rp,&s.Horizon,&s.CameraElevation,&s.Perspective,&hi,&ns,&dg,&os,&ld,&s.CreatedAt,&s.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows) { writeErr(w,404,"composition_not_found","composition spec not found"); return }
	if err != nil { writeErr(w,500,"composition_load_failed","could not load composition"); return }
	_ = json.Unmarshal(fp,&s.FocalPoints); _ = json.Unmarshal(bb,&s.BoundingBoxes); _ = json.Unmarshal(rp,&s.RelativePositions); _ = json.Unmarshal(hi,&s.Hierarchy); _ = json.Unmarshal(ns,&s.NegativeSpace); _ = json.Unmarshal(dg,&s.DominantGeometry); _ = json.Unmarshal(os,&s.ObjectScale); _ = json.Unmarshal(ld,&s.LightDirection)
	writeJSON(w,200,s)
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.PrincipalFromContext(r.Context()); if !ok { writeErr(w,401,"unauthorized","authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id")); if err != nil { writeErr(w,400,"invalid_project_id","invalid project id"); return }
	var in struct { CompositionSpecID *uuid.UUID `json:"composition_spec_id"`; SourceSimilarityCheckID *uuid.UUID `json:"source_similarity_check_id"`; CompositionSimilarity float64 `json:"composition_similarity"` }
	if err := decode(r,&in); err != nil { writeErr(w,400,"invalid_request","invalid mutation payload"); return }
	if in.CompositionSimilarity < 0 || in.CompositionSimilarity > 1 { writeErr(w,400,"invalid_score","composition_similarity must be between 0 and 1"); return }
	if in.CompositionSimilarity < 0.8 { writeErr(w,400,"mutation_not_needed","composition similarity is below the suggestion threshold"); return }
	suggestions := []string{"change focal point","shift horizon","change camera elevation","alter perspective","change object scale","change object placement","add/remove foreground obstruction","change negative space","change visual hierarchy","change light direction or dominant geometry"}
	raw,_ := json.Marshal(suggestions)
	var m Mutation; var stored []byte
	err=h.db.QueryRowContext(r.Context(),`INSERT INTO composition_mutations(project_id,user_id,composition_spec_id,source_similarity_check_id,suggestions,status) VALUES($1,$2,$3,$4,$5,'proposed') RETURNING id,project_id,user_id,composition_spec_id,source_similarity_check_id,suggestions,status,accepted_iteration_id,created_at`,pid,u.UserID,in.CompositionSpecID,in.SourceSimilarityCheckID,raw).Scan(&m.ID,&m.ProjectID,&m.UserID,&m.CompositionSpecID,&m.SourceSimilarityCheckID,&stored,&m.Status,&m.AcceptedIterationID,&m.CreatedAt)
	if err != nil { writeErr(w,500,"mutation_persist_failed","could not persist composition suggestions"); return }
	_ = json.Unmarshal(stored,&m.Suggestions); writeJSON(w,201,m)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.PrincipalFromContext(r.Context()); if !ok { writeErr(w,401,"unauthorized","authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id")); if err != nil { writeErr(w,400,"invalid_project_id","invalid project id"); return }
	mid, err := uuid.Parse(r.PathValue("mutation_id")); if err != nil { writeErr(w,400,"invalid_mutation_id","invalid mutation id"); return }
	tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeErr(w,500,"mutation_accept_failed","could not start transaction");return};defer tx.Rollback()
	var status string;var specID *uuid.UUID
	if err=tx.QueryRowContext(r.Context(),`SELECT status,composition_spec_id FROM composition_mutations WHERE id=$1 AND project_id=$2 AND user_id=$3 FOR UPDATE`,mid,pid,u.UserID).Scan(&status,&specID);errors.Is(err,sql.ErrNoRows){writeErr(w,404,"mutation_not_found","mutation suggestion not found");return};if err!=nil{writeErr(w,500,"mutation_load_failed","could not load mutation");return};if status!="proposed"{writeErr(w,409,"mutation_already_decided","mutation suggestion is already decided");return}
	var parentID uuid.UUID
	if specID!=nil{_ = tx.QueryRowContext(r.Context(),`SELECT iteration_id FROM composition_specs WHERE id=$1 AND project_id=$2`,*specID,pid).Scan(&parentID)}
	if parentID==uuid.Nil{if err:=tx.QueryRowContext(r.Context(),`SELECT id FROM iterations WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`,pid).Scan(&parentID);err!=nil{writeErr(w,409,"iteration_required","a source iteration is required");return}}
	var iterationID uuid.UUID
	if err=tx.QueryRowContext(r.Context(),`INSERT INTO iterations(project_id,parent_iteration_id,type,title,description) VALUES($1,$2,'composition',$3,$4) RETURNING id`,pid,parentID,"Composition mutation accepted","Accepted AI composition suggestion").Scan(&iterationID);err!=nil{writeErr(w,500,"iteration_create_failed","could not create accepted iteration");return}
	if _,err=tx.ExecContext(r.Context(),`UPDATE composition_mutations SET status='accepted',accepted_iteration_id=$1 WHERE id=$2`,iterationID,mid);err!=nil{writeErr(w,500,"mutation_update_failed","could not update mutation");return}
	if err=tx.Commit();err!=nil{writeErr(w,500,"mutation_commit_failed","could not commit mutation");return}
	var m Mutation
	var stored []byte
	payload:=map[string]any{"mutation_id":mid,"accepted_iteration_id":iterationID}
	if h.events!=nil{_,_=h.events.Append(r.Context(),u.UserID,pid,"iteration.created","iteration",iterationID,payload)}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u.UserID,ProjectID:pid,IterationID:&iterationID,EntityType:"composition_mutation",EntityID:mid,Action:"composition.mutation.accepted",Payload:payload})}
	if h.actions!=nil{_,_=h.actions.Create(r.Context(),u.UserID,pid,humanactions.Request{IterationID:&iterationID,ActionType:"COMPOSITION_MUTATION_ACCEPTED",Payload:payload,OldState:map[string]any{"status":"proposed","mutation_id":mid},NewState:map[string]any{"status":"accepted","mutation_id":mid,"accepted_iteration_id":iterationID},AIInfluence:map[string]any{"accepted":true}})}
	_ = h.db.QueryRowContext(r.Context(),`SELECT id,project_id,user_id,composition_spec_id,source_similarity_check_id,suggestions,status,accepted_iteration_id,created_at FROM composition_mutations WHERE id=$1`,mid).Scan(&m.ID,&m.ProjectID,&m.UserID,&m.CompositionSpecID,&m.SourceSimilarityCheckID,&stored,&m.Status,&m.AcceptedIterationID,&m.CreatedAt)
	_ = json.Unmarshal(stored,&m.Suggestions); writeJSON(w,200,m)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.PrincipalFromContext(r.Context()); if !ok { writeErr(w,401,"unauthorized","authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id")); if err != nil { writeErr(w,400,"invalid_project_id","invalid project id"); return }
	mid, err := uuid.Parse(r.PathValue("mutation_id")); if err != nil { writeErr(w,400,"invalid_mutation_id","invalid mutation id"); return }
	var m Mutation; var s []byte
	err=h.db.QueryRowContext(r.Context(),`UPDATE composition_mutations SET status='rejected' WHERE id=$1 AND project_id=$2 AND user_id=$3 AND status='proposed' RETURNING id,project_id,user_id,composition_spec_id,source_similarity_check_id,suggestions,status,accepted_iteration_id,created_at`,mid,pid,u.UserID).Scan(&m.ID,&m.ProjectID,&m.UserID,&m.CompositionSpecID,&m.SourceSimilarityCheckID,&s,&m.Status,&m.AcceptedIterationID,&m.CreatedAt)
	if errors.Is(err,sql.ErrNoRows){writeErr(w,409,"mutation_already_decided","mutation not found or already decided");return};if err!=nil{writeErr(w,500,"mutation_reject_failed","could not reject mutation");return};_=json.Unmarshal(s,&m.Suggestions)
	payload:=map[string]any{"mutation_id":mid,"status":"rejected"}
	if h.events!=nil{_,_=h.events.Append(r.Context(),u.UserID,pid,"composition.mutation.rejected","composition_mutation",mid,payload)}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u.UserID,ProjectID:pid,EntityType:"composition_mutation",EntityID:mid,Action:"composition.mutation.rejected",Payload:payload})}
	if h.actions!=nil{_,_=h.actions.Create(r.Context(),u.UserID,pid,humanactions.Request{ActionType:"AI_RECOMMENDATION_REJECTED",Payload:payload,OldState:map[string]any{"status":"proposed","mutation_id":mid},NewState:map[string]any{"status":"rejected","mutation_id":mid},AIInfluence:map[string]any{"source":"composition_mutation","rejected":true}})}
	writeJSON(w,200,m)
}

func decode(r *http.Request,v any) error {
	d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields()
	if err:=d.Decode(v);err!=nil{return err}
	var extra any
	if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")}
	return nil
}
func writeJSON(w http.ResponseWriter,s int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(s);_=json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,s int,c,m string){writeJSON(w,s,map[string]any{"error":map[string]string{"code":c,"message":m,"request_id":uuid.NewString()}})}
