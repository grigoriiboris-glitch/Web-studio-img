package recipes

import (
  "context"
  "crypto/sha256"
  "database/sql"
  "encoding/hex"
  "encoding/json"
  "errors"
  "fmt"
  "net/http"
  "regexp"
  "sort"
  "strings"
  "time"

  "github.com/google/uuid"

  "github.com/oleg3190/Web-studio-img/backend/internal/auth"
  "github.com/oleg3190/Web-studio-img/backend/internal/generation"
)

var ErrNotFound = errors.New("recipe not found")

type Parameter struct {
  Name string `json:"name"`
  Type string `json:"type"`
  Required bool `json:"required"`
  Path string `json:"path"`
  Default any `json:"default,omitempty"`
  Description string `json:"description,omitempty"`
}

type RecipeVersion struct {
  ID uuid.UUID `json:"id"`
  RecipeID uuid.UUID `json:"recipe_id"`
  Version int `json:"version"`
  Workflow map[string]any `json:"workflow"`
  InputMappings map[string]string `json:"input_mappings"`
  ExposedParameters []Parameter `json:"exposed_parameters"`
  DefaultParameters map[string]any `json:"default_parameters"`
  WorkflowHash string `json:"workflow_hash"`
  CreatedBy uuid.UUID `json:"created_by"`
  CreatedAt time.Time `json:"created_at"`
}

type Recipe struct {
  ID uuid.UUID `json:"id"`
  UserID uuid.UUID `json:"user_id"`
  ProjectID *uuid.UUID `json:"project_id,omitempty"`
  Name string `json:"name"`
  Description string `json:"description,omitempty"`
  Provider string `json:"provider"`
  Model string `json:"model"`
  ModelVersion string `json:"model_version,omitempty"`
  Scope string `json:"scope"`
  Tags []string `json:"tags"`
  PreviewAssetID *uuid.UUID `json:"preview_asset_id,omitempty"`
  Published bool `json:"published"`
  CurrentVersion int `json:"current_version"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
  Version *RecipeVersion `json:"version,omitempty"`
}

type VersionInput struct {
  Workflow map[string]any `json:"workflow"`
  InputMappings map[string]string `json:"input_mappings"`
  ExposedParameters []Parameter `json:"exposed_parameters"`
  DefaultParameters map[string]any `json:"default_parameters"`
}

type CreateInput struct {
  Name string `json:"name"`
  Description string `json:"description,omitempty"`
  Provider string `json:"provider"`
  Model string `json:"model"`
  ModelVersion string `json:"model_version,omitempty"`
  Scope string `json:"scope"`
  Tags []string `json:"tags"`
  ProjectID *uuid.UUID `json:"project_id,omitempty"`
  PreviewAssetID *uuid.UUID `json:"preview_asset_id,omitempty"`
  Version VersionInput `json:"version"`
}

type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) *Handler { return &Handler{db: db} }

func (h *Handler) Register(mux *http.ServeMux) {
  mux.HandleFunc("GET /api/v1/projects/{project_id}/recipes", h.list)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes", h.create)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/recipes/{recipe_id}", h.get)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes/{recipe_id}/versions", h.createVersion)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes/{recipe_id}/publish", h.publish)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes/{recipe_id}/unpublish", h.unpublish)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes/{recipe_id}/duplicate", h.duplicate)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes/{recipe_id}/compatibility", h.compatibility)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/recipes/from-generation/{generation_id}/create", h.fromGeneration)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
  uid,pid,ok:=h.auth(w,r); if !ok{return}
  rows,err:=h.db.QueryContext(r.Context(),`SELECT id,user_id,project_id,name,description,provider,model,COALESCE(model_version,''),scope,tags,published,current_version,created_at,updated_at FROM recipes WHERE (user_id=$1 AND project_id=$2) OR (user_id=$1 AND scope='global') OR (scope='global' AND published=TRUE) ORDER BY updated_at DESC`,uid,pid)
  if err!=nil{writeErr(w,500,"recipe_list_failed","could not list recipes");return}
  defer func(){_=rows.Close()}()
  out:=make([]Recipe,0)
  for rows.Next(){var x Recipe;var tags []byte;if err:=rows.Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Name,&x.Description,&x.Provider,&x.Model,&x.ModelVersion,&x.Scope,&tags,&x.Published,&x.CurrentVersion,&x.CreatedAt,&x.UpdatedAt);err!=nil{writeErr(w,500,"recipe_list_failed","could not read recipes");return};_=json.Unmarshal(tags,&x.Tags);out=append(out,x)}
  if err:=rows.Err();err!=nil{writeErr(w,500,"recipe_list_failed","could not read recipes");return}
  writeJSON(w,200,map[string]any{"recipes":out})
}

func (h *Handler) create(w http.ResponseWriter,r *http.Request) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};var in CreateInput
  if err:=decode(r,&in);err!=nil{writeErr(w,400,"invalid_recipe","invalid recipe payload");return}
  if err:=validateCreate(in,pid);err!=nil{writeErr(w,400,"invalid_recipe",err.Error());return}
  x,err:=h.createRecipe(r.Context(),uid,pid,in);if err!=nil{writeErr(w,409,"recipe_create_failed",err.Error());return};writeJSON(w,201,x)
}

func (h *Handler) get(w http.ResponseWriter,r *http.Request) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};id,err:=uuid.Parse(r.PathValue("recipe_id"));if err!=nil{writeErr(w,400,"invalid_recipe_id","invalid recipe id");return}
  x,err:=h.load(r.Context(),uid,pid,id);if errors.Is(err,ErrNotFound){writeErr(w,404,"recipe_not_found","recipe not found");return};if err!=nil{writeErr(w,500,"recipe_get_failed","could not load recipe");return};writeJSON(w,200,x)
}

func (h *Handler) createVersion(w http.ResponseWriter,r *http.Request) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};id,err:=uuid.Parse(r.PathValue("recipe_id"));if err!=nil{writeErr(w,400,"invalid_recipe_id","invalid recipe id");return}
  var in VersionInput;if err:=decode(r,&in);err!=nil{writeErr(w,400,"invalid_recipe_version","invalid version payload");return};if err:=validateVersion(in);err!=nil{writeErr(w,400,"invalid_recipe_version",err.Error());return}
  hash,err:=workflowHash(in.Workflow);if err!=nil{writeErr(w,400,"invalid_workflow",err.Error());return}
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeErr(w,500,"recipe_version_failed","could not start transaction");return};defer func(){_=tx.Rollback()}()
  var current int
  if err=tx.QueryRowContext(r.Context(),`SELECT current_version FROM recipes WHERE id=$1 AND user_id=$2 AND (project_id=$3 OR scope='global') FOR UPDATE`,id,uid,pid).Scan(&current);err!=nil{writeErr(w,404,"recipe_not_found","recipe not found");return}
  v:=current+1;wr,_:=json.Marshal(in.Workflow);mr,_:=json.Marshal(in.InputMappings);er,_:=json.Marshal(in.ExposedParameters);dr,_:=json.Marshal(in.DefaultParameters)
  if _,err=tx.ExecContext(r.Context(),`INSERT INTO recipe_versions(recipe_id,version,workflow,input_mappings,exposed_parameters,default_parameters,workflow_hash,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,id,v,wr,mr,er,dr,hash,uid);err!=nil{writeErr(w,500,"recipe_version_failed","could not create recipe version");return}
  if _,err=tx.ExecContext(r.Context(),`UPDATE recipes SET current_version=$1,published=FALSE,updated_at=now() WHERE id=$2`,v,id);err!=nil{writeErr(w,500,"recipe_version_failed","could not update recipe");return}
  if err=tx.Commit();err!=nil{writeErr(w,500,"recipe_version_failed","could not commit recipe version");return}
  x,err:=h.load(r.Context(),uid,pid,id);if err!=nil{writeErr(w,500,"recipe_get_failed","could not reload recipe");return};writeJSON(w,201,x)
}

func (h *Handler) publish(w http.ResponseWriter,r *http.Request){h.setPublished(w,r,true)}
func (h *Handler) unpublish(w http.ResponseWriter,r *http.Request){h.setPublished(w,r,false)}
func (h *Handler) setPublished(w http.ResponseWriter,r *http.Request,p bool) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};id,err:=uuid.Parse(r.PathValue("recipe_id"));if err!=nil{writeErr(w,400,"invalid_recipe_id","invalid recipe id");return}
  res,err:=h.db.ExecContext(r.Context(),`UPDATE recipes SET published=$1,updated_at=now() WHERE id=$2 AND user_id=$3 AND (project_id=$4 OR scope='global')`,p,id,uid,pid);if err!=nil{writeErr(w,500,"recipe_publish_failed","could not update recipe publication");return}
  n,_:=res.RowsAffected();if n==0{writeErr(w,404,"recipe_not_found","recipe not found");return};x,err:=h.load(r.Context(),uid,pid,id);if err!=nil{writeErr(w,500,"recipe_get_failed","could not reload recipe");return};writeJSON(w,200,x)
}

func (h *Handler) duplicate(w http.ResponseWriter,r *http.Request) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};id,err:=uuid.Parse(r.PathValue("recipe_id"));if err!=nil{writeErr(w,400,"invalid_recipe_id","invalid recipe id");return}
  src,err:=h.load(r.Context(),uid,pid,id);if err!=nil{writeErr(w,404,"recipe_not_found","recipe not found");return};if src.Version==nil{writeErr(w,409,"recipe_version_missing","recipe has no version");return}
  in:=CreateInput{Name:src.Name+" Copy",Description:src.Description,Provider:src.Provider,Model:src.Model,ModelVersion:src.ModelVersion,Scope:"project",ProjectID:&pid,Tags:src.Tags,Version:VersionInput{Workflow:src.Version.Workflow,InputMappings:src.Version.InputMappings,ExposedParameters:src.Version.ExposedParameters,DefaultParameters:src.Version.DefaultParameters}}
  x,err:=h.createRecipe(r.Context(),uid,pid,in);if err!=nil{writeErr(w,409,"recipe_duplicate_failed",err.Error());return};writeJSON(w,201,x)
}

func (h *Handler) compatibility(w http.ResponseWriter,r *http.Request) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};id,err:=uuid.Parse(r.PathValue("recipe_id"));if err!=nil{writeErr(w,400,"invalid_recipe_id","invalid recipe id");return}
  x,err:=h.load(r.Context(),uid,pid,id);if err!=nil{writeErr(w,404,"recipe_not_found","recipe not found");return}
  issues:=[]string{};if x.Provider!="comfyui"{issues=append(issues,"current backend provider is ComfyUI")};if x.Version==nil{issues=append(issues,"recipe has no current version")}else if err:=validateVersion(VersionInput{Workflow:x.Version.Workflow,InputMappings:x.Version.InputMappings,ExposedParameters:x.Version.ExposedParameters,DefaultParameters:x.Version.DefaultParameters});err!=nil{issues=append(issues,err.Error())}
  writeJSON(w,200,map[string]any{"compatible":len(issues)==0,"issues":issues,"provider":"comfyui","model":x.Model,"version":x.CurrentVersion})
}

func (h *Handler) fromGeneration(w http.ResponseWriter,r *http.Request) {
  uid,pid,ok:=h.auth(w,r);if !ok{return};gid,err:=uuid.Parse(r.PathValue("generation_id"));if err!=nil{writeErr(w,400,"invalid_generation_id","invalid generation id");return}
  var provider,model,modelVersion,status string;var wfRaw,paramsRaw []byte
  err=h.db.QueryRowContext(r.Context(),`SELECT provider,model,COALESCE(model_version,''),status,COALESCE(resolved_workflow,'{}'::jsonb),COALESCE(final_parameters,parameters) FROM generations WHERE id=$1 AND user_id=$2 AND project_id=$3`,gid,uid,pid).Scan(&provider,&model,&modelVersion,&status,&wfRaw,&paramsRaw)
  if err!=nil{writeErr(w,404,"generation_not_found","generation not found");return};if status!="succeeded"{writeErr(w,409,"generation_not_successful","only successful generations can become recipes");return}
  var wf map[string]any;if err=json.Unmarshal(wfRaw,&wf);err!=nil||len(wf)==0{writeErr(w,409,"workflow_missing","generation has no resolved workflow");return}
  var params map[string]any;_=json.Unmarshal(paramsRaw,&params);name:="Saved generation "+time.Now().UTC().Format("20060102-150405")
  in:=CreateInput{Name:name,Description:"Saved from successful generation",Provider:provider,Model:model,ModelVersion:modelVersion,Scope:"project",ProjectID:&pid,Tags:[]string{"saved-from-generation"},Version:VersionInput{Workflow:wf,DefaultParameters:params}}
  x,err:=h.createRecipe(r.Context(),uid,pid,in);if err!=nil{writeErr(w,409,"recipe_from_generation_failed",err.Error());return};writeJSON(w,201,x)
}

func (h *Handler) Resolve(ctx context.Context,userID,projectID,recipeID uuid.UUID,version int,params map[string]any)(generation.RecipeResolution,error) {
  if version < 1 { return generation.RecipeResolution{}, errors.New("recipe version is required") }
  x,err:=h.load(ctx,userID,projectID,recipeID,version);if err!=nil{return generation.RecipeResolution{},err};if x.Version==nil{return generation.RecipeResolution{},ErrNotFound}
  final:=map[string]any{};for k,v:=range x.Version.DefaultParameters{final[k]=v};for k,v:=range params{final[k]=v}
  for _,p:=range x.Version.ExposedParameters{if p.Required{v,ok:=final[p.Name];if !ok||v==nil||strings.TrimSpace(fmt.Sprint(v))==""{return generation.RecipeResolution{},fmt.Errorf("required recipe parameter %s is missing",p.Name)}}}
  wf:=deepCopy(x.Version.Workflow);for name,path:=range x.Version.InputMappings{if value,ok:=final[name];ok{if err:=setPath(wf,path,value);err!=nil{return generation.RecipeResolution{},fmt.Errorf("recipe parameter %s: %w",name,err)}}}
  hash,_:=workflowHash(wf)
  return generation.RecipeResolution{RecipeID:x.ID,Version:x.Version.Version,Provider:x.Provider,Model:x.Model,ModelVersion:x.ModelVersion,ResolvedWorkflow:wf,ResolvedWorkflowHash:hash,FinalParameters:final},nil
}

func (h *Handler) createRecipe(ctx context.Context,uid,pid uuid.UUID,in CreateInput)(*Recipe,error) {
  if err:=validateCreate(in,pid);err!=nil{return nil,err};hash,err:=workflowHash(in.Version.Workflow);if err!=nil{return nil,err}
  tx,err:=h.db.BeginTx(ctx,nil);if err!=nil{return nil,err};defer func(){_=tx.Rollback()}()
  tags,_:=json.Marshal(uniqueStrings(in.Tags));wr,_:=json.Marshal(in.Version.Workflow);mr,_:=json.Marshal(in.Version.InputMappings);er,_:=json.Marshal(in.Version.ExposedParameters);dr,_:=json.Marshal(in.Version.DefaultParameters)
  var id uuid.UUID
  err=tx.QueryRowContext(ctx,`INSERT INTO recipes(user_id,project_id,name,description,provider,model,model_version,scope,tags,preview_asset_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,uid,in.ProjectID,strings.TrimSpace(in.Name),strings.TrimSpace(in.Description),in.Provider,in.Model,in.ModelVersion,in.Scope,tags,in.PreviewAssetID).Scan(&id);if err!=nil{return nil,err}
  if _,err=tx.ExecContext(ctx,`INSERT INTO recipe_versions(recipe_id,version,workflow,input_mappings,exposed_parameters,default_parameters,workflow_hash,created_by) VALUES($1,1,$2,$3,$4,$5,$6,$7)`,id,wr,mr,er,dr,hash,uid);err!=nil{return nil,err}
  if err=tx.Commit();err!=nil{return nil,err};return h.load(ctx,uid,pid,id)
}

func (h *Handler) load(ctx context.Context,uid,pid,id uuid.UUID,requestedVersion ...int)(*Recipe,error) {
  var x Recipe;var tags []byte
  recipeQuery := `SELECT id,user_id,project_id,name,description,provider,model,COALESCE(model_version,''),scope,tags,preview_asset_id,published,current_version,created_at,updated_at FROM recipes WHERE id=$1 AND ((user_id=$2 AND project_id=$3) OR (scope='global' AND published=TRUE))`
  err:=h.db.QueryRowContext(ctx,recipeQuery,id,uid,pid).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Name,&x.Description,&x.Provider,&x.Model,&x.ModelVersion,&x.Scope,&tags,&x.PreviewAssetID,&x.Published,&x.CurrentVersion,&x.CreatedAt,&x.UpdatedAt)
  if errors.Is(err,sql.ErrNoRows){return nil,ErrNotFound};if err!=nil{return nil,err};_=json.Unmarshal(tags,&x.Tags)
  version:=x.CurrentVersion;if len(requestedVersion)>0&&requestedVersion[0]>0{version=requestedVersion[0]}
  var v RecipeVersion;var wf,m,e,d []byte
  err=h.db.QueryRowContext(ctx,`SELECT id,recipe_id,version,workflow,input_mappings,exposed_parameters,default_parameters,workflow_hash,created_by,created_at FROM recipe_versions WHERE recipe_id=$1 AND version=$2`,id,version).Scan(&v.ID,&v.RecipeID,&v.Version,&wf,&m,&e,&d,&v.WorkflowHash,&v.CreatedBy,&v.CreatedAt)
  if errors.Is(err,sql.ErrNoRows){return nil,ErrNotFound};if err!=nil{return nil,err}
  _=json.Unmarshal(wf,&v.Workflow);_=json.Unmarshal(m,&v.InputMappings);_=json.Unmarshal(e,&v.ExposedParameters);_=json.Unmarshal(d,&v.DefaultParameters);x.Version=&v
  return &x,nil
}

func validateCreate(in CreateInput,pid uuid.UUID)error {
  if strings.TrimSpace(in.Name)==""||strings.TrimSpace(in.Provider)==""||strings.TrimSpace(in.Model)==""{return errors.New("name, provider and model are required")}
  if in.Scope==""{in.Scope="project"};if in.Scope=="project"{if in.ProjectID!=nil&&*in.ProjectID!=pid{return errors.New("project mismatch")}}else if in.Scope!="global"{return errors.New("scope must be project or global")}
  return validateVersion(in.Version)
}

func validateVersion(v VersionInput)error {
  if len(v.Workflow)==0{return errors.New("workflow is required")};if err:=rejectSecrets(v.Workflow);err!=nil{return err}
  seen:=map[string]bool{}
  for _,p:=range v.ExposedParameters{
    if strings.TrimSpace(p.Name)==""||strings.TrimSpace(p.Path)==""{return errors.New("exposed parameters require name and path")}
    if seen[p.Name]{return errors.New("duplicate exposed parameter "+p.Name)}
    seen[p.Name]=true
    if !map[string]bool{"string":true,"number":true,"integer":true,"boolean":true,"image":true,"mask":true}[p.Type]{return errors.New("unsupported parameter type "+p.Type)}
    mappedPath,ok:=v.InputMappings[p.Name];if !ok{return fmt.Errorf("exposed parameter %s is not mapped",p.Name)}
    if mappedPath!=p.Path{return fmt.Errorf("exposed parameter %s mapping must match path",p.Name)}
    if !workflowPathExists(v.Workflow,mappedPath){return fmt.Errorf("exposed parameter %s path %s does not exist in workflow",p.Name,mappedPath)}
  }
  for name,path:=range v.InputMappings{if strings.TrimSpace(name)==""||!strings.HasPrefix(path,"/"){return errors.New("input mapping paths must be JSON Pointer paths")};if !workflowPathExists(v.Workflow,path){return fmt.Errorf("input mapping %s points to a missing workflow path",name)}}
  return nil
}

var secretKeyRE=regexp.MustCompile(`(?i)(secret|token|api[_-]?key|password|authorization|credential|private[_-]?key)`)
func rejectSecrets(value any)error {
  switch x:=value.(type){case map[string]any:for k,v:=range x{if secretKeyRE.MatchString(k){return fmt.Errorf("workflow contains forbidden secret-like key %q",k)};if err:=rejectSecrets(v);err!=nil{return err}}
  case []any:for _,v:=range x{if err:=rejectSecrets(v);err!=nil{return err}}};return nil
}

func workflowPathExists(root map[string]any,path string) bool {
  if !strings.HasPrefix(path,"/") { return false }
  parts:=strings.Split(strings.TrimPrefix(path,"/"),"/")
  var cur any=root
  for _,raw:=range parts {
    part:=strings.ReplaceAll(strings.ReplaceAll(raw,"~1","/"),"~0","~")
    m,ok:=cur.(map[string]any); if !ok { return false }
    next,ok:=m[part]; if !ok { return false }
    cur=next
  }
  return true
}

func setPath(root map[string]any,path string,value any)error {
  parts:=strings.Split(strings.TrimPrefix(path,"/"),"/");var cur any=root
  for i,part:=range parts{part=strings.ReplaceAll(strings.ReplaceAll(part,"~1","/"),"~0","~");m,ok:=cur.(map[string]any);if !ok{return errors.New("path does not point to object")};if i==len(parts)-1{if _,exists:=m[part];!exists{return fmt.Errorf("workflow path %s does not exist",path)};m[part]=value;return nil};next,ok:=m[part];if !ok{return fmt.Errorf("workflow path %s does not exist",path)};cur=next};return nil
}

func workflowHash(v any)(string,error){b,err:=json.Marshal(v);if err!=nil{return "",err};sum:=sha256.Sum256(b);return hex.EncodeToString(sum[:]),nil}
func deepCopy(v map[string]any)map[string]any{b,_:=json.Marshal(v);var out map[string]any;_=json.Unmarshal(b,&out);return out}
func uniqueStrings(in []string)[]string{seen:=map[string]bool{};out:=make([]string,0,len(in));for _,v:=range in{v=strings.TrimSpace(v);if v!=""&&!seen[v]{seen[v]=true;out=append(out,v)}};sort.Strings(out);return out}
func decode(r *http.Request,v any)error{defer func(){ _ = r.Body.Close() }();d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();return d.Decode(v)}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message}})}
func (h *Handler) auth(w http.ResponseWriter,r *http.Request)(uuid.UUID,uuid.UUID,bool){principal,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeErr(w,401,"unauthorized","authentication required");return uuid.Nil,uuid.Nil,false};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return uuid.Nil,uuid.Nil,false};return principal.UserID,pid,true}
