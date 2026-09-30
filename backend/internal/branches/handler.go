package branches

import (
    "encoding/json"
    "errors"
    "io"
    "net/http"

    "github.com/google/uuid"
    "github.com/oleg3190/Web-studio-img/backend/internal/auth"
    "github.com/oleg3190/Web-studio-img/backend/internal/events"
    "github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Handler struct{ store *Store; events *events.Store; provenance *provenance.Store }
func NewHandler(store *Store,eventStore *events.Store,provenanceStore *provenance.Store)(*Handler,error){if store==nil{return nil,errors.New("branch handler requires store")};return &Handler{store:store,events:eventStore,provenance:provenanceStore},nil}
func(h *Handler)Register(mux *http.ServeMux){
    mux.HandleFunc("GET /api/v1/projects/{project_id}/branches",h.list)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/branches",h.create)
    mux.HandleFunc("PATCH /api/v1/projects/{project_id}/branches/{branch_id}",h.update)
    mux.HandleFunc("GET /api/v1/projects/{project_id}/branches/compare",h.compare)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/branches/{branch_id}/merge",h.merge)
}
func user(r *http.Request)(uuid.UUID,bool){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{return uuid.Nil,false};return p.UserID,true}
func id(v string)(uuid.UUID,error){return uuid.Parse(v)}
func(h *Handler)list(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{errJSON(w,401,"unauthorized");return};p,e:=id(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id");return};v,e:=h.store.List(r.Context(),u,p);if errors.Is(e,ErrBranchNotFound){errJSON(w,404,"project_not_found");return};if e!=nil{errJSON(w,500,"branch_list_failed");return};jsonOut(w,200,map[string]any{"branches":v})}
func(h *Handler)create(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{errJSON(w,401,"unauthorized");return};p,e:=id(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id");return};var in struct{Name string `json:"name"`;ParentIterationID *uuid.UUID `json:"parent_iteration_id,omitempty"`};if !decode(r,&in){errJSON(w,400,"invalid_request");return};v,e:=h.store.Create(r.Context(),u,p,in.ParentIterationID,in.Name);if errors.Is(e,ErrInvalidBranch){errJSON(w,400,"invalid_branch");return};if errors.Is(e,ErrBranchNotFound){errJSON(w,404,"branch_parent_or_project_not_found");return};if e!=nil{errJSON(w,409,"branch_create_failed");return};h.record(r,u,p,v.ID,"branch.created",map[string]any{"name":v.Name,"parent_iteration_id":v.ParentIterationID});jsonOut(w,201,v)}
func(h *Handler)update(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{errJSON(w,401,"unauthorized");return};p,e:=id(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id");return};b,e:=id(r.PathValue("branch_id"));if e!=nil{errJSON(w,400,"invalid_branch_id");return};var in struct{Name string `json:"name"`;Status Status `json:"status"`};if !decode(r,&in){errJSON(w,400,"invalid_request");return};v,e:=h.store.Update(r.Context(),u,p,b,in.Name,in.Status);if errors.Is(e,ErrBranchNotFound){errJSON(w,404,"branch_not_found");return};if errors.Is(e,ErrInvalidBranch){errJSON(w,400,"invalid_branch");return};if e!=nil{errJSON(w,409,"branch_update_failed");return};h.record(r,u,p,v.ID,"branch.updated",map[string]any{"name":v.Name,"status":v.Status});jsonOut(w,200,v)}
func(h *Handler)compare(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{errJSON(w,401,"unauthorized");return};p,e:=id(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id");return};a,e:=id(r.URL.Query().Get("source_branch_id"));if e!=nil{errJSON(w,400,"invalid_source_branch_id");return};b,e:=id(r.URL.Query().Get("target_branch_id"));if e!=nil{errJSON(w,400,"invalid_target_branch_id");return};v,e:=h.store.Compare(r.Context(),u,p,a,b);if errors.Is(e,ErrBranchNotFound){errJSON(w,404,"branch_not_found");return};if e!=nil{errJSON(w,409,"branch_compare_failed");return};jsonOut(w,200,map[string]any{"differences":v})}
func(h *Handler)merge(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{errJSON(w,401,"unauthorized");return};p,e:=id(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id");return};t,e:=id(r.PathValue("branch_id"));if e!=nil{errJSON(w,400,"invalid_target_branch_id");return};var in MergeRequest;if !decode(r,&in){errJSON(w,400,"invalid_request");return};v,e:=h.store.Merge(r.Context(),u,p,t,in);if errors.Is(e,ErrMergeConflict){errJSON(w,409,"merge_conflict");return};if errors.Is(e,ErrBranchArchived){errJSON(w,409,"branch_archived");return};if errors.Is(e,ErrBranchNotFound){errJSON(w,404,"branch_not_found");return};if e!=nil{errJSON(w,500,"merge_failed");return};h.record(r,u,p,v.ID,"branch.merged",map[string]any{"target_branch_id":t,"source_branch_ids":in.Sources,"decisions":in.Decisions});jsonOut(w,201,v)}
func(h *Handler)record(r *http.Request,u,p,e uuid.UUID,action string,payload map[string]any){if h.events!=nil{_,_=h.events.Append(r.Context(),u,p,action,"branch",e,payload)};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:p,IterationID:&e,EntityType:"branch",EntityID:e,Action:action,Payload:payload})}}
func decode(r *http.Request,v any)bool{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if d.Decode(v)!=nil{return false};var extra any;return d.Decode(&extra)==io.EOF}
func jsonOut(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func errJSON(w http.ResponseWriter,status,code string){jsonOut(w,status,map[string]any{"error":map[string]string{"code":code,"request_id":uuid.NewString()}})}
