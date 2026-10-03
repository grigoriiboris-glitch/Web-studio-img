package printprofiles

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

const maxJSONBytes = 1 << 20
var keyPattern = regexp.MustCompile("^[a-z0-9][a-z0-9_-]{1,63}$")

type Profile struct {
  ID uuid.UUID `json:"id"`
  ProjectID uuid.UUID `json:"project_id"`
  Key string `json:"key"`
  Name string `json:"name"`
  CurrentVersion int `json:"current_version"`
  ArchivedAt *time.Time `json:"archived_at,omitempty"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}
type Version struct {
  ID uuid.UUID `json:"id"`
  PrintProfileID uuid.UUID `json:"print_profile_id"`
  Version int `json:"version"`
  Spec map[string]any `json:"spec"`
  CreatedAt time.Time `json:"created_at"`
}
type createInput struct {
  Key string `json:"key"`
  Name string `json:"name"`
  Spec map[string]any `json:"spec"`
}
type updateInput struct {
  Name *string `json:"name"`
  Spec map[string]any `json:"spec"`
}
type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) (*Handler, error) {
  if db == nil { return nil, errors.New("print profile handler requires database")
  }
  return &Handler{db: db}, nil
}
func (h *Handler) Register(mux *http.ServeMux) {
  mux.HandleFunc("GET /api/v1/projects/{project_id}/print-profiles", h.list)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/print-profiles", h.create)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/print-profiles/{print_profile_id}", h.get)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/print-profiles/{print_profile_id}/versions", h.versions)
  mux.HandleFunc("PATCH /api/v1/projects/{project_id}/print-profiles/{print_profile_id}", h.update)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/print-profiles/{print_profile_id}/archive", h.archive)
}
func (h *Handler) project(w http.ResponseWriter,r *http.Request)(uuid.UUID,bool) {
  principal,ok:=auth.PrincipalFromContext(r.Context()); if !ok { writeError(w,401,"unauthorized","authentication required"); return uuid.Nil,false }
  id,err:=uuid.Parse(r.PathValue("project_id")); if err!=nil { writeError(w,400,"invalid_project_id","invalid project id"); return uuid.Nil,false }
  var exists bool
  if err:=h.db.QueryRowContext(r.Context(),"SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')",id,principal.UserID).Scan(&exists);err!=nil||!exists {writeError(w,404,"project_not_found","project not found");return uuid.Nil,false}
  return id,true
}
func (h *Handler) ids(w http.ResponseWriter,r *http.Request)(uuid.UUID,uuid.UUID,bool){p,ok:=h.project(w,r);if !ok{return uuid.Nil,uuid.Nil,false};id,err:=uuid.Parse(r.PathValue("print_profile_id"));if err!=nil{writeError(w,400,"invalid_print_profile_id","invalid print profile id");return uuid.Nil,uuid.Nil,false};return p,id,true}
func (h *Handler) list(w http.ResponseWriter,r *http.Request){p,ok:=h.project(w,r);if !ok{return};rows,err:=h.db.QueryContext(r.Context(),"SELECT id,project_id,key,name,current_version,archived_at,created_at,updated_at FROM print_profiles WHERE project_id=$1 ORDER BY archived_at NULLS FIRST,name,key",p);if err!=nil{writeError(w,500,"print_profile_list_failed","could not list print profiles");return};defer func(){_ = rows.Close()};out:=[]Profile{};for rows.Next(){var x Profile;if err:=rows.Scan(&x.ID,&x.ProjectID,&x.Key,&x.Name,&x.CurrentVersion,&x.ArchivedAt,&x.CreatedAt,&x.UpdatedAt);err!=nil{writeError(w,500,"print_profile_list_failed","could not read print profiles");return};out=append(out,x)};if err:=rows.Err();err!=nil{writeError(w,500,"print_profile_list_failed","could not read print profiles");return};writeJSON(w,200,map[string]any{"print_profiles":out})}
func (h *Handler) create(w http.ResponseWriter,r *http.Request){p,ok:=h.project(w,r);if !ok{return};var in createInput;if err:=decode(r,&in);err!=nil{writeError(w,400,"invalid_request","invalid print profile payload");return};in.Key=strings.TrimSpace(strings.ToLower(in.Key));in.Name=strings.TrimSpace(in.Name);if !keyPattern.MatchString(in.Key){writeError(w,400,"invalid_print_profile_key","key must be 2-64 chars: lowercase letters, digits, _ or -");return};if in.Name==""||len([]rune(in.Name))>200{writeError(w,400,"invalid_print_profile_name","name is required and must be <= 200 characters");return};if err:=validateSpec(in.Spec);err!=nil{writeError(w,400,"invalid_print_profile",err.Error());return};tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"print_profile_create_failed","could not start transaction");return};defer func(){_ = tx.Rollback()};var x Profile;err=tx.QueryRowContext(r.Context(),"INSERT INTO print_profiles(project_id,key,name) VALUES($1,$2,$3) RETURNING id,project_id,key,name,current_version,archived_at,created_at,updated_at",p,in.Key,in.Name).Scan(&x.ID,&x.ProjectID,&x.Key,&x.Name,&x.CurrentVersion,&x.ArchivedAt,&x.CreatedAt,&x.UpdatedAt);if err!=nil{if strings.Contains(err.Error(),"print_profiles_project_key_uq"){writeError(w,409,"print_profile_key_conflict","a print profile with this key already exists");return};writeError(w,500,"print_profile_create_failed","could not create print profile");return};v,err:=insertVersion(r.Context(),tx,x.ID,1,in.Spec);if err!=nil{writeError(w,500,"print_profile_create_failed","could not create print profile version");return};if err:=tx.Commit();err!=nil{writeError(w,500,"print_profile_create_failed","could not commit print profile");return};writeJSON(w,201,map[string]any{"print_profile":x,"version":v})}
func (h *Handler) get(w http.ResponseWriter,r *http.Request){p,id,ok:=h.ids(w,r);if !ok{return};x,err:=h.loadProfile(r.Context(),p,id);if errors.Is(err,sql.ErrNoRows){writeError(w,404,"print_profile_not_found","print profile not found");return};if err!=nil{writeError(w,500,"print_profile_get_failed","could not load print profile");return};v,err:=h.loadVersion(r.Context(),id,x.CurrentVersion);if err!=nil{writeError(w,500,"print_profile_get_failed","could not load current print profile version");return};writeJSON(w,200,map[string]any{"print_profile":x,"version":v})}
func (h *Handler) versions(w http.ResponseWriter,r *http.Request){p,id,ok:=h.ids(w,r);if !ok{return};if _,err:=h.loadProfile(r.Context(),p,id);errors.Is(err,sql.ErrNoRows){writeError(w,404,"print_profile_not_found","print profile not found");return}else if err!=nil{writeError(w,500,"print_profile_versions_failed","could not load print profile");return};rows,err:=h.db.QueryContext(r.Context(),"SELECT id,print_profile_id,version,spec,created_at FROM print_profile_versions WHERE print_profile_id=$1 ORDER BY version DESC",id);if err!=nil{writeError(w,500,"print_profile_versions_failed","could not list print profile versions");return};defer func(){_ = rows.Close()};out:=[]Version{};for rows.Next(){v,err:=scanVersion(rows);if err!=nil{writeError(w,500,"print_profile_versions_failed","could not read print profile versions");return};out=append(out,v)};writeJSON(w,200,map[string]any{"versions":out})}
func (h *Handler) update(w http.ResponseWriter,r *http.Request){p,id,ok:=h.ids(w,r);if !ok{return};var in updateInput;if err:=decode(r,&in);err!=nil{writeError(w,400,"invalid_request","invalid print profile update");return};if err:=validateSpec(in.Spec);err!=nil{writeError(w,400,"invalid_print_profile",err.Error());return};tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"print_profile_update_failed","could not start transaction");return};defer func(){_ = tx.Rollback()};var x Profile;var current int;err=tx.QueryRowContext(r.Context(),"SELECT id,project_id,key,name,current_version,archived_at,created_at,updated_at FROM print_profiles WHERE id=$1 AND project_id=$2 FOR UPDATE",id,p).Scan(&x.ID,&x.ProjectID,&x.Key,&x.Name,&current,&x.ArchivedAt,&x.CreatedAt,&x.UpdatedAt);if errors.Is(err,sql.ErrNoRows){writeError(w,404,"print_profile_not_found","print profile not found");return};if err!=nil{writeError(w,500,"print_profile_update_failed","could not load print profile");return};if x.ArchivedAt!=nil{writeError(w,409,"print_profile_archived","archived print profile cannot be changed");return};if in.Name!=nil{x.Name=strings.TrimSpace(*in.Name);if x.Name==""||len([]rune(x.Name))>200{writeError(w,400,"invalid_print_profile_name","name is required and must be <= 200 characters");return}};v,err:=insertVersion(r.Context(),tx,id,current+1,in.Spec);if err!=nil{writeError(w,500,"print_profile_update_failed","could not create print profile version");return};if _,err=tx.ExecContext(r.Context(),"UPDATE print_profiles SET name=$2,current_version=$3,updated_at=now() WHERE id=$1",id,x.Name,current+1);err!=nil{writeError(w,500,"print_profile_update_failed","could not publish print profile version");return};if err=tx.Commit();err!=nil{writeError(w,500,"print_profile_update_failed","could not commit print profile version");return};x.CurrentVersion=current+1;x.UpdatedAt=time.Now().UTC();writeJSON(w,200,map[string]any{"print_profile":x,"version":v})}
func (h *Handler) archive(w http.ResponseWriter,r *http.Request){p,id,ok:=h.ids(w,r);if !ok{return};res,err:=h.db.ExecContext(r.Context(),"UPDATE print_profiles SET archived_at=COALESCE(archived_at,now()),updated_at=now() WHERE id=$1 AND project_id=$2",id,p);if err!=nil{writeError(w,500,"print_profile_archive_failed","could not archive print profile");return};if n,_:=res.RowsAffected();n==0{writeError(w,404,"print_profile_not_found","print profile not found");return};w.WriteHeader(204)}
func (h *Handler) loadProfile(ctx context.Context,p,id uuid.UUID)(Profile,error){var x Profile;err:=h.db.QueryRowContext(ctx,"SELECT id,project_id,key,name,current_version,archived_at,created_at,updated_at FROM print_profiles WHERE id=$1 AND project_id=$2",id,p).Scan(&x.ID,&x.ProjectID,&x.Key,&x.Name,&x.CurrentVersion,&x.ArchivedAt,&x.CreatedAt,&x.UpdatedAt);return x,err}
func (h *Handler) loadVersion(ctx context.Context,id uuid.UUID,n int)(Version,error){return scanVersion(h.db.QueryRowContext(ctx,"SELECT id,print_profile_id,version,spec,created_at FROM print_profile_versions WHERE print_profile_id=$1 AND version=$2",id,n))}
type scanner interface{Scan(...any)error}
func scanVersion(row scanner)(Version,error){var v Version;var raw []byte;if err:=row.Scan(&v.ID,&v.PrintProfileID,&v.Version,&raw,&v.CreatedAt);err!=nil{return v,err};if err:=json.Unmarshal(raw,&v.Spec);err!=nil{return v,err};return v,nil}
func insertVersion(ctx context.Context,tx *sql.Tx,id uuid.UUID,n int,spec map[string]any)(Version,error){raw,_:=json.Marshal(spec);var v Version;var stored []byte;err:=tx.QueryRowContext(ctx,"INSERT INTO print_profile_versions(print_profile_id,version,spec) VALUES($1,$2,$3::jsonb) RETURNING id,print_profile_id,version,spec,created_at",id,n,raw).Scan(&v.ID,&v.PrintProfileID,&v.Version,&stored,&v.CreatedAt);if err!=nil{return v,err};if err:=json.Unmarshal(stored,&v.Spec);err!=nil{return v,err};return v,nil}
func validateSpec(spec map[string]any)error{if spec==nil{return errors.New("spec is required")};raw,err:=json.Marshal(spec);if err!=nil||len(raw)>maxJSONBytes{return errors.New("spec is not valid JSON")};return nil}
func decode(r *http.Request,target any)error{d:=json.NewDecoder(io.LimitReader(r.Body,maxJSONBytes));d.DisallowUnknownFields();if err:=d.Decode(target);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")};return nil}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message,"request_id":uuid.NewString()}})}
