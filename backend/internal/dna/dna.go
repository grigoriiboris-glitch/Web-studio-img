package dna

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type AssetProfile struct {
	ID uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name string `json:"name"`
	Features map[string]any `json:"features"`
	SourceIterations []string `json:"source_iterations"`
	SourceAssets []string `json:"source_assets"`
	Reusable bool `json:"reusable"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type VisualLanguage struct {
	ID uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
	Name string `json:"name"`
	Signals map[string]any `json:"signals"`
	SourceIterations []string `json:"source_iterations"`
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

func NewHandler(db *sql.DB, actions *humanactions.Store, provenanceStore *provenance.Store) (*Handler,error) {
	if db==nil { return nil,errors.New("dna handler requires database") }
	return &Handler{db:db,actions:actions,provenance:provenanceStore},nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/asset-dna",h.listAssets)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/asset-dna/analyze",h.analyzeAsset)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/visual-language",h.listLanguage)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/visual-language/analyze",h.analyzeLanguage)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/visual-language/{profile_id}/apply",h.applyLanguage)
}

func currentUserID(r *http.Request)(uuid.UUID,bool){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{return uuid.Nil,false};return p.UserID,true}
func decode(r *http.Request,v any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple json values")};return nil}
func (h *Handler) ownedProject(ctx context.Context,u,p uuid.UUID)bool{var ok bool;return h.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`,p,u).Scan(&ok)==nil&&ok}

func (h *Handler) listAssets(w http.ResponseWriter,r *http.Request){u,ok:=currentUserID(r);if !ok{errJSON(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){errJSON(w,404,"project_not_found","project not found");return};rows,e:=h.db.QueryContext(r.Context(),`SELECT id,user_id,project_id,name,features,source_iterations,source_assets,reusable,created_at,updated_at FROM asset_dna_profiles WHERE user_id=$1 AND project_id=$2 ORDER BY updated_at DESC LIMIT 100`,u,p);if e!=nil{errJSON(w,500,"asset_dna_list_failed","could not list asset DNA");return};defer rows.Close();out:=[]AssetProfile{};for rows.Next(){var x AssetProfile;var f,it,as []byte;if e:=rows.Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Name,&f,&it,&as,&x.Reusable,&x.CreatedAt,&x.UpdatedAt);e!=nil{errJSON(w,500,"asset_dna_list_failed","could not read asset DNA");return};_ = json.Unmarshal(f,&x.Features);_=json.Unmarshal(it,&x.SourceIterations);_=json.Unmarshal(as,&x.SourceAssets);if x.Features==nil{x.Features=map[string]any{}};out=append(out,x)};if e:=rows.Err();e!=nil{errJSON(w,500,"asset_dna_list_failed","could not read asset DNA");return};writeJSON(w,200,map[string]any{"profiles":out})}

func (h *Handler) analyzeAsset(w http.ResponseWriter,r *http.Request){u,ok:=currentUserID(r);if !ok{errJSON(w,401,"unauthorized","authentication required");return};pid,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,pid){errJSON(w,404,"project_not_found","project not found");return}
	materials:=map[string]int{};textures:=map[string]int{};actions:=map[string]int{};var sources []string
	rows,e:=h.db.QueryContext(r.Context(),`SELECT id::text,COALESCE(iteration_id::text,''),action_type,COALESCE(payload,'{}') FROM human_actions WHERE project_id=$1 AND user_id=$2 ORDER BY version ASC`,pid,u);if e!=nil{errJSON(w,500,"asset_dna_analyze_failed","could not read human actions");return};defer rows.Close()
	for rows.Next(){var id,iterationID,typ string;var raw []byte;if e:=rows.Scan(&id,&iterationID,&typ,&raw);e!=nil{errJSON(w,500,"asset_dna_analyze_failed","could not read human actions");return};actions[typ]++;if iterationID!=""{sources=append(sources,iterationID)};var p map[string]any;_=json.Unmarshal(raw,&p);if typ=="MATERIAL_SELECTED"{if v,ok:=p["material"].(string);ok&&v!=""{materials[v]++}};if typ=="TEXTURE_SELECTED"{if v,ok:=p["texture"].(string);ok&&v!=""{textures[v]++}}}
	sort.Strings(sources);features:=map[string]any{"materials":materials,"textures":textures,"actions":actions,"source":"project creative history","uncertainty":"Deterministic summary of recorded project choices; features are descriptive and reusable only with user intent."};f,_:=json.Marshal(features);it,_:=json.Marshal(sources);as,_:=json.Marshal([]string{});name:="Project Asset DNA";var x AssetProfile
	e=h.db.QueryRowContext(r.Context(),`INSERT INTO asset_dna_profiles(user_id,project_id,name,features,source_iterations,source_assets) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,user_id,project_id,name,features,source_iterations,source_assets,reusable,created_at,updated_at`,u,pid,name,f,it,as).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Name,&f,&it,&as,&x.Reusable,&x.CreatedAt,&x.UpdatedAt);if e!=nil{errJSON(w,500,"asset_dna_analyze_failed","could not save asset DNA");return};_=json.Unmarshal(f,&x.Features);_=json.Unmarshal(it,&x.SourceIterations);_=json.Unmarshal(as,&x.SourceAssets);h.audit(r,u,pid,x.ID,"ASSET_DNA_CREATED",map[string]any{"profile_id":x.ID,"source_action_count":len(sources)});writeJSON(w,201,x)}

func (h *Handler) listLanguage(w http.ResponseWriter,r *http.Request){u,ok:=currentUserID(r);if !ok{errJSON(w,401,"unauthorized","authentication required");return};rows,e:=h.db.QueryContext(r.Context(),`SELECT id,user_id,project_id,name,signals,source_iterations,version,prompt_influence,created_at,updated_at FROM visual_language_profiles WHERE user_id=$1 AND project_id IS NULL ORDER BY updated_at DESC LIMIT 100`,u);if e!=nil{errJSON(w,500,"visual_language_list_failed","could not list visual language profiles");return};defer rows.Close();out:=[]VisualLanguage{};for rows.Next(){var x VisualLanguage;var s,it []byte;if e:=rows.Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Name,&s,&it,&x.Version,&x.PromptInfluence,&x.CreatedAt,&x.UpdatedAt);e!=nil{errJSON(w,500,"visual_language_list_failed","could not read visual language profiles");return};_=json.Unmarshal(s,&x.Signals);_=json.Unmarshal(it,&x.SourceIterations);if x.Signals==nil{x.Signals=map[string]any{}};out=append(out,x)};writeJSON(w,200,map[string]any{"profiles":out})}

func (h *Handler) analyzeLanguage(w http.ResponseWriter,r *http.Request){u,ok:=currentUserID(r);if !ok{errJSON(w,401,"unauthorized","authentication required");return}
	materials:=map[string]int{};textures:=map[string]int{};actions:=map[string]int{};sources:=[]string{}
	rows,e:=h.db.QueryContext(r.Context(),`SELECT h.id::text,h.action_type,COALESCE(h.payload,'{}') FROM human_actions h JOIN projects p ON p.id=h.project_id WHERE h.user_id=$1 AND p.user_id=$1 AND p.status <> 'deleted' ORDER BY h.created_at ASC`,u);if e!=nil{errJSON(w,500,"visual_language_analyze_failed","could not read creative history");return};defer rows.Close()
	for rows.Next(){var id,typ string;var raw []byte;if e:=rows.Scan(&id,&typ,&raw);e!=nil{errJSON(w,500,"visual_language_analyze_failed","could not read creative history");return};sources=append(sources,id);actions[typ]++;var p map[string]any;_=json.Unmarshal(raw,&p);if typ=="MATERIAL_SELECTED"{if v,ok:=p["material"].(string);ok&&v!=""{materials[v]++}};if typ=="TEXTURE_SELECTED"{if v,ok:=p["texture"].(string);ok&&v!=""{textures[v]++}}}
	signals:=map[string]any{"materials":materials,"textures":textures,"actions":actions,"source":"all owned project human choices","uncertainty":"Aggregated from recorded user choices only; it does not infer identity or copy external artists."};s,_:=json.Marshal(signals);it,_:=json.Marshal(sources);var x VisualLanguage;e=h.db.QueryRowContext(r.Context(),`INSERT INTO visual_language_profiles(user_id,name,signals,source_iterations,prompt_influence) VALUES($1,'Personal Visual Language',$2,$3,false) RETURNING id,user_id,project_id,name,signals,source_iterations,version,prompt_influence,created_at,updated_at`,u,s,it).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Name,&s,&it,&x.Version,&x.PromptInfluence,&x.CreatedAt,&x.UpdatedAt);if e!=nil{errJSON(w,500,"visual_language_analyze_failed","could not save visual language");return};_=json.Unmarshal(s,&x.Signals);_=json.Unmarshal(it,&x.SourceIterations);h.audit(r,u,uuid.Nil,x.ID,"VISUAL_LANGUAGE_UPDATED",map[string]any{"profile_id":x.ID,"source_action_count":len(sources)});writeJSON(w,201,x)}

func (h *Handler) applyLanguage(w http.ResponseWriter,r *http.Request){u,ok:=currentUserID(r);if !ok{errJSON(w,401,"unauthorized","authentication required");return};pid,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,pid){errJSON(w,404,"project_not_found","project not found");return};id,e:=uuid.Parse(r.PathValue("profile_id"));if e!=nil{errJSON(w,400,"invalid_profile_id","invalid profile id");return};var signals []byte;var influence bool;if e=h.db.QueryRowContext(r.Context(),`SELECT signals,prompt_influence FROM visual_language_profiles WHERE id=$1 AND user_id=$2 AND project_id IS NULL`,id,u).Scan(&signals,&influence);errors.Is(e,sql.ErrNoRows){errJSON(w,404,"profile_not_found","visual language profile not found");return}else if e!=nil{errJSON(w,500,"visual_language_apply_failed","could not load profile");return};if !influence{errJSON(w,409,"visual_language_not_enabled","prompt influence is disabled for this profile");return};var out map[string]any;_=json.Unmarshal(signals,&out);h.audit(r,u,pid,id,"VISUAL_LANGUAGE_UPDATED",map[string]any{"profile_id":id,"project_id":pid,"automatic_prompt_mutation":false});writeJSON(w,200,map[string]any{"profile_id":id,"prompt_suggestion":out,"automatic_prompt_mutation":false,"user_must_approve":true})}

func(h *Handler) audit(r *http.Request,u,pid,id uuid.UUID,action string,payload map[string]any){if pid!=uuid.Nil&&h.actions!=nil{_,_=h.actions.Create(r.Context(),u,pid,humanactions.Request{ActionType:action,Payload:payload})};if pid!=uuid.Nil&&h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:pid,EntityType:"creative_profile",EntityID:id,Action:strings.ToLower(action),Payload:payload})}}
func writeJSON(w http.ResponseWriter,s int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(s);_=json.NewEncoder(w).Encode(v)}
func errJSON(w http.ResponseWriter,s,c,m string){writeJSON(w,s,map[string]any{"error":map[string]string{"code":c,"message":m,"request_id":uuid.NewString()}})}
