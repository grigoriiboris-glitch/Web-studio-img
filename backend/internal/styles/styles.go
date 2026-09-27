package styles

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Profile struct {
	ID uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	Name string `json:"name"`
	Description *string `json:"description,omitempty"`
	Parameters map[string]any `json:"parameters"`
	Version int `json:"version"`
	PromptInfluence bool `json:"prompt_influence"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Handler struct {
	db *sql.DB
	actions *humanactions.Store
	provenance *provenance.Store
}

func NewHandler(db *sql.DB, actions *humanactions.Store, provenanceStore *provenance.Store) (*Handler, error) {
	if db == nil { return nil, errors.New("styles handler requires database") }
	return &Handler{db: db, actions: actions, provenance: provenanceStore}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/style-profiles", h.listGlobal)
	mux.HandleFunc("POST /api/v1/style-profiles", h.createGlobal)
	mux.HandleFunc("GET /api/v1/style-profiles/{profile_id}", h.getGlobal)
	mux.HandleFunc("PATCH /api/v1/style-profiles/{profile_id}", h.updateGlobal)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/style-profiles", h.listProject)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/style-profiles/infer", h.infer)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/style-profiles/{profile_id}/apply", h.apply)
}

type input struct {
	Name string `json:"name"`
	Description *string `json:"description,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
	PromptInfluence *bool `json:"prompt_influence,omitempty"`
}

func currentUserID(r *http.Request) (uuid.UUID, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { return uuid.Nil, false }
	return p.UserID, true
}

func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil { return err }
	var extra any
	if err := d.Decode(&extra); err != io.EOF { return errors.New("multiple json values") }
	return nil
}

func (h *Handler) ownedProject(ctx context.Context, userID, projectID uuid.UUID) bool {
	var ok bool
	return h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`, projectID,userID).Scan(&ok)==nil && ok
}

func (h *Handler) listGlobal(w http.ResponseWriter, r *http.Request) {
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
	h.list(w,r,userID)
}
func (h *Handler) listProject(w http.ResponseWriter,r *http.Request) {
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
	pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil||!h.ownedProject(r.Context(),userID,pid){writeErr(w,404,"project_not_found","project not found");return}
	h.list(w,r,userID)
}
func (h *Handler) list(w http.ResponseWriter,r *http.Request,userID uuid.UUID){
	rows,err:=h.db.QueryContext(r.Context(),`SELECT id,user_id,name,description,parameters,version,prompt_influence,created_at,updated_at FROM style_profiles WHERE user_id=$1 ORDER BY updated_at DESC LIMIT 100`,userID)
	if err!=nil{writeErr(w,500,"style_profile_list_failed","could not list style profiles");return};defer rows.Close()
	out:=[]Profile{}
	for rows.Next(){var p Profile;var raw []byte;if err:=rows.Scan(&p.ID,&p.UserID,&p.Name,&p.Description,&raw,&p.Version,&p.PromptInfluence,&p.CreatedAt,&p.UpdatedAt);err!=nil{writeErr(w,500,"style_profile_list_failed","could not read style profiles");return};_ = json.Unmarshal(raw,&p.Parameters);if p.Parameters==nil{p.Parameters=map[string]any{}};out=append(out,p)}
	if err:=rows.Err();err!=nil{writeErr(w,500,"style_profile_list_failed","could not read style profiles");return};writeJSON(w,200,map[string]any{"profiles":out})
}
func (h *Handler) createGlobal(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};var in input;if err:=decode(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid style profile payload");return}
	if strings.TrimSpace(in.Name)==""||len([]rune(in.Name))>200{writeErr(w,400,"invalid_style_profile","name is required");return}
	params:=in.Parameters;if params==nil{params=map[string]any{}};raw,_:=json.Marshal(params);var p Profile
	err=h.db.QueryRowContext(r.Context(),`INSERT INTO style_profiles(user_id,name,description,parameters,prompt_influence) VALUES($1,$2,$3,$4,COALESCE($5,false)) RETURNING id,user_id,name,description,parameters,version,prompt_influence,created_at,updated_at`,userID,strings.TrimSpace(in.Name),in.Description,raw,in.PromptInfluence).Scan(&p.ID,&p.UserID,&p.Name,&p.Description,&raw,&p.Version,&p.PromptInfluence,&p.CreatedAt,&p.UpdatedAt)
	if err!=nil{writeErr(w,500,"style_profile_create_failed","could not create style profile");return};_ = json.Unmarshal(raw,&p.Parameters);writeJSON(w,201,p)
}
func (h *Handler) getGlobal(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};id,err:=uuid.Parse(r.PathValue("profile_id"));if err!=nil{writeErr(w,400,"invalid_profile_id","invalid profile id");return}
	var p Profile;var raw []byte;err=h.db.QueryRowContext(r.Context(),`SELECT id,user_id,name,description,parameters,version,prompt_influence,created_at,updated_at FROM style_profiles WHERE id=$1 AND user_id=$2`,id,userID).Scan(&p.ID,&p.UserID,&p.Name,&p.Description,&raw,&p.Version,&p.PromptInfluence,&p.CreatedAt,&p.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){writeErr(w,404,"profile_not_found","style profile not found");return};if err!=nil{writeErr(w,500,"style_profile_get_failed","could not load style profile");return};_ = json.Unmarshal(raw,&p.Parameters);if p.Parameters==nil{p.Parameters=map[string]any{}};writeJSON(w,200,p)
}
func (h *Handler) updateGlobal(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};id,err:=uuid.Parse(r.PathValue("profile_id"));if err!=nil{writeErr(w,400,"invalid_profile_id","invalid profile id");return};var in input;if err=decode(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid style profile payload");return}
	var p Profile;var raw []byte;if err=h.db.QueryRowContext(r.Context(),`SELECT id,user_id,name,description,parameters,version,prompt_influence,created_at,updated_at FROM style_profiles WHERE id=$1 AND user_id=$2`,id,userID).Scan(&p.ID,&p.UserID,&p.Name,&p.Description,&raw,&p.Version,&p.PromptInfluence,&p.CreatedAt,&p.UpdatedAt);errors.Is(err,sql.ErrNoRows){writeErr(w,404,"profile_not_found","style profile not found");return}else if err!=nil{writeErr(w,500,"style_profile_get_failed","could not load style profile");return}
	if strings.TrimSpace(in.Name)==""{in.Name=p.Name};if in.Description==nil{in.Description=p.Description};if in.Parameters==nil{_ = json.Unmarshal(raw,&in.Parameters)};if in.PromptInfluence==nil{v:=p.PromptInfluence;in.PromptInfluence=&v}
	raw,_=json.Marshal(in.Parameters)
	err=h.db.QueryRowContext(r.Context(),`UPDATE style_profiles SET name=$1,description=$2,parameters=$3,version=version+1,prompt_influence=$4,updated_at=now() WHERE id=$5 AND user_id=$6 RETURNING id,user_id,name,description,parameters,version,prompt_influence,created_at,updated_at`,strings.TrimSpace(in.Name),in.Description,raw,*in.PromptInfluence,id,userID).Scan(&p.ID,&p.UserID,&p.Name,&p.Description,&raw,&p.Version,&p.PromptInfluence,&p.CreatedAt,&p.UpdatedAt)
	if err!=nil{writeErr(w,500,"style_profile_update_failed","could not update style profile");return};_ = json.Unmarshal(raw,&p.Parameters);writeJSON(w,200,p)
}
func (h *Handler) infer(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil||!h.ownedProject(r.Context(),userID,pid){writeErr(w,404,"project_not_found","project not found");return}
	materials:=map[string]int{};textures:=map[string]int{};actions:=map[string]int{}
	rows,queryErr:=h.db.QueryContext(r.Context(),`SELECT action_type,COALESCE(payload,'{}') FROM human_actions WHERE project_id=$1 AND user_id=$2 ORDER BY version ASC`,pid,userID);if queryErr!=nil{writeErr(w,500,"style_profile_infer_failed","could not read creative history");return};defer rows.Close()
	for rows.Next(){var typ string;var raw []byte;if err:=rows.Scan(&typ,&raw);err!=nil{writeErr(w,500,"style_profile_infer_failed","could not read creative history");return};actions[typ]++;var p map[string]any;_=json.Unmarshal(raw,&p);if typ=="MATERIAL_SELECTED"{if v,ok:=p["material"].(string);ok&&v!=""{materials[v]++}};if typ=="TEXTURE_SELECTED"{if v,ok:=p["texture"].(string);ok&&v!=""{textures[v]++}}}
	params:=map[string]any{"source":"project-human-actions","materials":materials,"textures":textures,"actions":actions,"uncertainty":"Descriptive deterministic inference from recorded human choices; it does not copy a named artist or external work."};raw,_:=json.Marshal(params);var p Profile
	err=h.db.QueryRowContext(r.Context(),`INSERT INTO style_profiles(user_id,name,description,parameters,prompt_influence) VALUES($1,'Inferred visual preferences',$2,$3,false) RETURNING id,user_id,name,description,parameters,version,prompt_influence,created_at,updated_at`,userID,"Derived from this project's recorded creative choices",raw).Scan(&p.ID,&p.UserID,&p.Name,&p.Description,&raw,&p.Version,&p.PromptInfluence,&p.CreatedAt,&p.UpdatedAt);if queryErr!=nil{writeErr(w,500,"style_profile_infer_failed","could not save inferred profile");return};_ = json.Unmarshal(raw,&p.Parameters);h.audit(r,userID,pid,p.ID,"STYLE_PROFILE_CREATED",map[string]any{"profile_id":p.ID,"project_id":pid,"inferred":true});writeJSON(w,201,p)
}
func (h *Handler) apply(w http.ResponseWriter,r *http.Request){
	userID,ok:=currentUserID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil||!h.ownedProject(r.Context(),userID,pid){writeErr(w,404,"project_not_found","project not found");return};id,err:=uuid.Parse(r.PathValue("profile_id"));if err!=nil{writeErr(w,400,"invalid_profile_id","invalid profile id");return}
	var name string;var params []byte;var influence bool;if err=h.db.QueryRowContext(r.Context(),`SELECT name,parameters,prompt_influence FROM style_profiles WHERE id=$1 AND user_id=$2`,id,userID).Scan(&name,&params,&influence);errors.Is(err,sql.ErrNoRows){writeErr(w,404,"profile_not_found","style profile not found");return}else if err!=nil{writeErr(w,500,"style_profile_apply_failed","could not load style profile");return};if !influence{writeErr(w,409,"style_profile_not_enabled","enable prompt influence before applying");return}
	var suggestion map[string]any;_ = json.Unmarshal(params,&suggestion);h.audit(r,userID,pid,id,"STYLE_PROFILE_APPLIED",map[string]any{"profile_id":id,"automatic_prompt_mutation":false});writeJSON(w,200,map[string]any{"profile_id":id,"name":name,"prompt_suggestion":suggestion,"applied":true,"automatic_prompt_mutation":false,"user_must_approve":true})
}
func (h *Handler) audit(r *http.Request,userID,projectID,entityID uuid.UUID,action string,payload map[string]any){
	if h.actions!=nil{_,_=h.actions.Create(r.Context(),userID,projectID,humanactions.Request{ActionType:action,Payload:payload})}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:userID,ProjectID:projectID,EntityType:"style_profile",EntityID:entityID,Action:strings.ToLower(action),Payload:payload})}
}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,status int,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}
