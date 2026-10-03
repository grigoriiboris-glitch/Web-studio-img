package cardtypes

import (
  "context"
  "database/sql"
  "encoding/json"
  "errors"
  "io"
  "net/http"
  "regexp"
  "strings"
  "time"

  "github.com/google/uuid"
  "github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

const maxJSONBytes = 2 << 20

var keyPattern = regexp.MustCompile("^[a-z0-9][a-z0-9_-]{1,63}$")

type Definition struct {
  ID uuid.UUID `json:"id"`
  ProjectID uuid.UUID `json:"project_id"`
  Key string `json:"key"`
  Name string `json:"name"`
  Description string `json:"description"`
  CurrentVersion int `json:"current_version"`
  ArchivedAt *time.Time `json:"archived_at,omitempty"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}

type Version struct {
  ID uuid.UUID `json:"id"`
  CardTypeID uuid.UUID `json:"card_type_id"`
  Version int `json:"version"`
  Schema map[string]any `json:"schema"`
  ProductionDefaults map[string]any `json:"production_defaults"`
  DefaultRecipeID *string `json:"default_recipe_id,omitempty"`
  DefaultRecipeVersion *int `json:"default_recipe_version,omitempty"`
  DefaultTemplateID *string `json:"default_template_id,omitempty"`
  DefaultTemplateVersion *int `json:"default_template_version,omitempty"`
  PromptRules map[string]any `json:"prompt_rules"`
  RejectReasonProfileID *uuid.UUID `json:"reject_reason_profile_id,omitempty"`
  CreatedAt time.Time `json:"created_at"`
}

type createInput struct {
  Key string `json:"key"`
  Name string `json:"name"`
  Description string `json:"description"`
  Schema map[string]any `json:"schema"`
  ProductionDefaults map[string]any `json:"production_defaults"`
  DefaultRecipeID *string `json:"default_recipe_id"`
  DefaultRecipeVersion *int `json:"default_recipe_version"`
  DefaultTemplateID *string `json:"default_template_id"`
  DefaultTemplateVersion *int `json:"default_template_version"`
  PromptRules map[string]any `json:"prompt_rules"`
  RejectReasonProfileID *uuid.UUID `json:"reject_reason_profile_id"`
}

type updateInput struct {
  Name *string `json:"name"`
  Description *string `json:"description"`
  Schema map[string]any `json:"schema"`
  ProductionDefaults map[string]any `json:"production_defaults"`
  DefaultRecipeID *string `json:"default_recipe_id"`
  DefaultRecipeVersion *int `json:"default_recipe_version"`
  DefaultTemplateID *string `json:"default_template_id"`
  DefaultTemplateVersion *int `json:"default_template_version"`
  PromptRules map[string]any `json:"prompt_rules"`
  RejectReasonProfileID *uuid.UUID `json:"reject_reason_profile_id"`
}

type Handler struct { db *sql.DB }

func NewHandler(db *sql.DB) (*Handler, error) {
  if db == nil { return nil, errors.New("card type handler requires database") }
  return &Handler{db: db}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
  mux.HandleFunc("GET /api/v1/projects/{project_id}/card-types", h.list)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/card-types", h.create)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/card-types/{card_type_id}", h.get)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/card-types/{card_type_id}/versions", h.versions)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/card-types/{card_type_id}/clone", h.clone)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/card-types/{card_type_id}/archive", h.archive)
  mux.HandleFunc("PATCH /api/v1/projects/{project_id}/card-types/{card_type_id}", h.update)
}

func (h *Handler) project(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
  principal, ok := auth.PrincipalFromContext(r.Context())
  if !ok { writeError(w, 401, "unauthorized", "authentication required"); return uuid.Nil, false }
  id, err := uuid.Parse(r.PathValue("project_id"))
  if err != nil { writeError(w, 400, "invalid_project_id", "invalid project id"); return uuid.Nil, false }
  var exists bool
  if err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')", id, principal.UserID).Scan(&exists); err != nil || !exists {
    writeError(w, 404, "project_not_found", "project not found"); return uuid.Nil, false
  }
  return id, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
  projectID, ok := h.project(w,r); if !ok{return}
  rows, err := h.db.QueryContext(r.Context(), `SELECT id,project_id,key,name,description,current_version,archived_at,created_at,updated_at FROM card_type_definitions WHERE project_id=$1 ORDER BY archived_at NULLS FIRST,name,key`, projectID)
  if err != nil { writeError(w,500,"card_type_list_failed","could not list card types"); return }
  defer func() { _ = rows.Close() }()
  out:=[]Definition{}
  for rows.Next(){ var d Definition; if err:=rows.Scan(&d.ID,&d.ProjectID,&d.Key,&d.Name,&d.Description,&d.CurrentVersion,&d.ArchivedAt,&d.CreatedAt,&d.UpdatedAt);err!=nil{writeError(w,500,"card_type_list_failed","could not read card types");return};out=append(out,d)}
  if err:=rows.Err();err!=nil{writeError(w,500,"card_type_list_failed","could not read card types");return}
  writeJSON(w,200,map[string]any{"card_types":out})
}

func (h *Handler) create(w http.ResponseWriter,r *http.Request){
  projectID,ok:=h.project(w,r);if !ok{return}
  var in createInput
  if err:=decode(r,&in);err!=nil{writeError(w,400,"invalid_request","invalid card type payload");return}
  in.Key=strings.TrimSpace(strings.ToLower(in.Key)); in.Name=strings.TrimSpace(in.Name)
  if !keyPattern.MatchString(in.Key){writeError(w,400,"invalid_card_type_key","key must be 2-64 chars: lowercase letters, digits, _ or -");return}
  if in.Name==""||len([]rune(in.Name))>200{writeError(w,400,"invalid_card_type_name","name is required and must be <= 200 characters");return}
  if err:=validateVersionPayload(in.Schema,in.ProductionDefaults,in.PromptRules);err!=nil{writeError(w,400,"invalid_card_type",err.Error());return}
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"card_type_create_failed","could not start transaction");return};defer func() { _ = tx.Rollback() }()
  var d Definition
  err=tx.QueryRowContext(r.Context(),`INSERT INTO card_type_definitions(project_id,key,name,description) VALUES($1,$2,$3,$4) RETURNING id,project_id,key,name,description,current_version,archived_at,created_at,updated_at`,projectID,in.Key,in.Name,in.Description).Scan(&d.ID,&d.ProjectID,&d.Key,&d.Name,&d.Description,&d.CurrentVersion,&d.ArchivedAt,&d.CreatedAt,&d.UpdatedAt)
  if err!=nil{if strings.Contains(err.Error(),"card_type_definitions_project_key_uq"){writeError(w,409,"card_type_key_conflict","a card type with this key already exists");return};writeError(w,500,"card_type_create_failed","could not create card type");return}
  v,err:=insertVersion(r.Context(),tx,d.ID,1,in);if err!=nil{writeError(w,500,"card_type_create_failed","could not create initial card type version");return}
  if err:=tx.Commit();err!=nil{writeError(w,500,"card_type_create_failed","could not commit card type");return}
  writeJSON(w,201,map[string]any{"card_type":d,"version":v})
}

func (h *Handler) get(w http.ResponseWriter,r *http.Request){
  projectID,id,ok:=h.ids(w,r);if !ok{return}
  d,err:=h.loadDefinition(r.Context(),projectID,id);if errors.Is(err,sql.ErrNoRows){writeError(w,404,"card_type_not_found","card type not found");return};if err!=nil{writeError(w,500,"card_type_get_failed","could not load card type");return}
  v,err:=h.loadVersion(r.Context(),id,d.CurrentVersion);if err!=nil{writeError(w,500,"card_type_get_failed","could not load current card type version");return}
  writeJSON(w,200,map[string]any{"card_type":d,"version":v})
}

func (h *Handler) versions(w http.ResponseWriter,r *http.Request){
  projectID,id,ok:=h.ids(w,r);if !ok{return}
  if _,err:=h.loadDefinition(r.Context(),projectID,id);errors.Is(err,sql.ErrNoRows){writeError(w,404,"card_type_not_found","card type not found");return}else if err!=nil{writeError(w,500,"card_type_versions_failed","could not load card type");return}
  rows,err:=h.db.QueryContext(r.Context(),`SELECT id,card_type_id,version,schema,production_defaults,default_recipe_id,default_recipe_version,default_template_id,default_template_version,prompt_rules,reject_reason_profile_id,created_at FROM card_type_versions WHERE card_type_id=$1 ORDER BY version DESC`,id)
  if err!=nil{writeError(w,500,"card_type_versions_failed","could not list versions");return};defer func() { _ = rows.Close() }()
  out:=[]Version{}
  for rows.Next(){v,err:=scanVersion(rows);if err!=nil{writeError(w,500,"card_type_versions_failed","could not read versions");return};out=append(out,v)}
  writeJSON(w,200,map[string]any{"versions":out})
}

func (h *Handler) update(w http.ResponseWriter,r *http.Request){
  projectID,id,ok:=h.ids(w,r);if !ok{return}
  var in updateInput;if err:=decode(r,&in);err!=nil{writeError(w,400,"invalid_request","invalid card type update");return}
  if err:=validateVersionPayload(in.Schema,in.ProductionDefaults,in.PromptRules);err!=nil{writeError(w,400,"invalid_card_type",err.Error());return}
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"card_type_update_failed","could not start transaction");return};defer func() { _ = tx.Rollback() }()
  var d Definition
  var current int
  err=tx.QueryRowContext(r.Context(),`SELECT id,project_id,key,name,description,current_version FROM card_type_definitions WHERE id=$1 AND project_id=$2 FOR UPDATE`,id,projectID).Scan(&d.ID,&d.ProjectID,&d.Key,&d.Name,&d.Description,&current)
  if errors.Is(err,sql.ErrNoRows){writeError(w,404,"card_type_not_found","card type not found");return};if err!=nil{writeError(w,500,"card_type_update_failed","could not load card type");return}
  if d.ArchivedAt!=nil{writeError(w,409,"card_type_archived","archived card type cannot be changed");return}
  if in.Name!=nil { d.Name=strings.TrimSpace(*in.Name); if d.Name==""||len([]rune(d.Name))>200{writeError(w,400,"invalid_card_type_name","name is required and must be <= 200 characters");return} }
  if in.Description!=nil { d.Description=*in.Description }
  versionInput:=createInput{Schema:in.Schema,ProductionDefaults:in.ProductionDefaults,DefaultRecipeID:in.DefaultRecipeID,DefaultRecipeVersion:in.DefaultRecipeVersion,DefaultTemplateID:in.DefaultTemplateID,DefaultTemplateVersion:in.DefaultTemplateVersion,PromptRules:in.PromptRules,RejectReasonProfileID:in.RejectReasonProfileID}
  if err:=validateVersionPayload(versionInput.Schema,versionInput.ProductionDefaults,versionInput.PromptRules);err!=nil{writeError(w,400,"invalid_card_type",err.Error());return}
  next:=current+1
  v,err:=insertVersion(r.Context(),tx,id,next,versionInput);if err!=nil{writeError(w,500,"card_type_update_failed","could not create card type version");return}
  if _,err=tx.ExecContext(r.Context(),"UPDATE card_type_definitions SET name=$2,description=$3,current_version=$4,updated_at=now() WHERE id=$1",id,d.Name,d.Description,next);err!=nil{writeError(w,500,"card_type_update_failed","could not publish card type version");return}
  if err=tx.Commit();err!=nil{writeError(w,500,"card_type_update_failed","could not commit card type version");return}
  d.CurrentVersion=next; d.UpdatedAt=time.Now().UTC()
  writeJSON(w,200,map[string]any{"card_type":d,"version":v})
}

func (h *Handler) clone(w http.ResponseWriter,r *http.Request){
  projectID,id,ok:=h.ids(w,r);if !ok{return}
  var in struct{Name string `json:"name"`;Key string `json:"key"`}
  if err:=decode(r,&in);err!=nil{writeError(w,400,"invalid_request","invalid clone payload");return}
  source,err:=h.loadDefinition(r.Context(),projectID,id);if errors.Is(err,sql.ErrNoRows){writeError(w,404,"card_type_not_found","card type not found");return};if err!=nil{writeError(w,500,"card_type_clone_failed","could not load source type");return}
  v,err:=h.loadVersion(r.Context(),id,source.CurrentVersion);if err!=nil{writeError(w,500,"card_type_clone_failed","could not load source version");return}
  name:=strings.TrimSpace(in.Name);if name==""{name=source.Name+" Copy"};key:=strings.TrimSpace(strings.ToLower(in.Key));if key==""{key=source.Key+"-copy"};if !keyPattern.MatchString(key){writeError(w,400,"invalid_card_type_key","invalid clone key");return}
  input:=createInput{Key:key,Name:name,Description:source.Description,Schema:v.Schema,ProductionDefaults:v.ProductionDefaults,DefaultRecipeID:v.DefaultRecipeID,DefaultRecipeVersion:v.DefaultRecipeVersion,DefaultTemplateID:v.DefaultTemplateID,DefaultTemplateVersion:v.DefaultTemplateVersion,PromptRules:v.PromptRules,RejectReasonProfileID:v.RejectReasonProfileID}
  body,_:=json.Marshal(input)
  req:=r.Clone(context.WithValue(r.Context(), clonePayloadKey{}, body))
  _=req
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"card_type_clone_failed","could not start transaction");return};defer func() { _ = tx.Rollback() }()
  var d Definition
  err=tx.QueryRowContext(r.Context(),`INSERT INTO card_type_definitions(project_id,key,name,description) VALUES($1,$2,$3,$4) RETURNING id,project_id,key,name,description,current_version,archived_at,created_at,updated_at`,projectID,key,name,input.Description).Scan(&d.ID,&d.ProjectID,&d.Key,&d.Name,&d.Description,&d.CurrentVersion,&d.ArchivedAt,&d.CreatedAt,&d.UpdatedAt)
  if err!=nil{if strings.Contains(err.Error(),"card_type_definitions_project_key_uq"){writeError(w,409,"card_type_key_conflict","a card type with this key already exists");return};writeError(w,500,"card_type_clone_failed","could not create clone");return}
  v.ID=uuid.Nil;v.CardTypeID=d.ID;v.Version=1;v.CreatedAt=time.Now().UTC()
  v,err=insertVersion(r.Context(),tx,d.ID,1,input);if err!=nil{writeError(w,500,"card_type_clone_failed","could not create clone version");return}
  if err=tx.Commit();err!=nil{writeError(w,500,"card_type_clone_failed","could not commit clone");return}
  writeJSON(w,201,map[string]any{"card_type":d,"version":v})
}

func (h *Handler) archive(w http.ResponseWriter,r *http.Request){
  projectID,id,ok:=h.ids(w,r);if !ok{return}
  result,err:=h.db.ExecContext(r.Context(),"UPDATE card_type_definitions SET archived_at=COALESCE(archived_at,now()),updated_at=now() WHERE id=$1 AND project_id=$2",id,projectID)
  if err!=nil{writeError(w,500,"card_type_archive_failed","could not archive card type");return};if n,_:=result.RowsAffected();n==0{writeError(w,404,"card_type_not_found","card type not found");return};w.WriteHeader(204)
}

func (h *Handler) ids(w http.ResponseWriter,r *http.Request)(uuid.UUID,uuid.UUID,bool){p,ok:=h.project(w,r);if !ok{return uuid.Nil,uuid.Nil,false};id,err:=uuid.Parse(r.PathValue("card_type_id"));if err!=nil{writeError(w,400,"invalid_card_type_id","invalid card type id");return uuid.Nil,uuid.Nil,false};return p,id,true}

func (h *Handler) loadDefinition(ctx context.Context,projectID,id uuid.UUID)(Definition,error){var d Definition;err:=h.db.QueryRowContext(ctx,"SELECT id,project_id,key,name,description,current_version,archived_at,created_at,updated_at FROM card_type_definitions WHERE id=$1 AND project_id=$2",id,projectID).Scan(&d.ID,&d.ProjectID,&d.Key,&d.Name,&d.Description,&d.CurrentVersion,&d.ArchivedAt,&d.CreatedAt,&d.UpdatedAt);return d,err}

func (h *Handler) loadVersion(ctx context.Context,id uuid.UUID,n int)(Version,error){return scanVersion(h.db.QueryRowContext(ctx,`SELECT id,card_type_id,version,schema,production_defaults,default_recipe_id,default_recipe_version,default_template_id,default_template_version,prompt_rules,reject_reason_profile_id,created_at FROM card_type_versions WHERE card_type_id=$1 AND version=$2`,id,n))}

func insertVersion(ctx context.Context,tx *sql.Tx,id uuid.UUID,n int,in createInput)(Version,error){
  var v Version;var schema,defaults,rules []byte
  schema=mustJSON(in.Schema);defaults=mustJSON(in.ProductionDefaults);rules=mustJSON(in.PromptRules)
  err:=tx.QueryRowContext(ctx,`INSERT INTO card_type_versions(card_type_id,version,schema,production_defaults,default_recipe_id,default_recipe_version,default_template_id,default_template_version,prompt_rules,reject_reason_profile_id) VALUES($1,$2,$3::jsonb,$4::jsonb,$5,$6,$7,$8,$9::jsonb,$10) RETURNING id,card_type_id,version,schema,production_defaults,default_recipe_id,default_recipe_version,default_template_id,default_template_version,prompt_rules,reject_reason_profile_id,created_at`,id,n,schema,defaults,in.DefaultRecipeID,in.DefaultRecipeVersion,in.DefaultTemplateID,in.DefaultTemplateVersion,rules,in.RejectReasonProfileID).Scan(versionArgs(&v,&schema,&defaults,&rules)...)
  if err!=nil{return Version{},err};_ = json.Unmarshal(schema,&v.Schema);_ = json.Unmarshal(defaults,&v.ProductionDefaults);_ = json.Unmarshal(rules,&v.PromptRules);return v,nil
}

type scanner interface{Scan(...any)error}
func scanVersion(row scanner)(Version,error){var v Version;var schema,defaults,rules []byte;err:=row.Scan(&v.ID,&v.CardTypeID,&v.Version,&schema,&defaults,&v.DefaultRecipeID,&v.DefaultRecipeVersion,&v.DefaultTemplateID,&v.DefaultTemplateVersion,&rules,&v.RejectReasonProfileID,&v.CreatedAt);if err!=nil{return v,err};if err=json.Unmarshal(schema,&v.Schema);err!=nil{return v,err};if err=json.Unmarshal(defaults,&v.ProductionDefaults);err!=nil{return v,err};if err=json.Unmarshal(rules,&v.PromptRules);err!=nil{return v,err};return v,nil}
func versionArgs(v *Version,schema,defaults,rules *[]byte)[]any{return []any{&v.ID,&v.CardTypeID,&v.Version,schema,defaults,&v.DefaultRecipeID,&v.DefaultRecipeVersion,&v.DefaultTemplateID,&v.DefaultTemplateVersion,rules,&v.RejectReasonProfileID,&v.CreatedAt}}
func validateVersionPayload(schema,defaults,rules map[string]any)error{for name,v:=range map[string]any{"schema":schema,"production_defaults":defaults,"prompt_rules":rules}{if v==nil{continue};b,err:=json.Marshal(v);if err!=nil||len(b)>maxJSONBytes{return errors.New(name+" is not valid JSON")}};return nil}
func mustJSON(v any)[]byte{b,_:=json.Marshal(v);return b}
func decode(r *http.Request,target any)error{decoder:=json.NewDecoder(io.LimitReader(r.Body,maxJSONBytes));decoder.DisallowUnknownFields();if err:=decoder.Decode(target);err!=nil{return err};var extra any;if err:=decoder.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")};return nil}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message,"request_id":uuid.NewString()}})}

type clonePayloadKey struct{}
