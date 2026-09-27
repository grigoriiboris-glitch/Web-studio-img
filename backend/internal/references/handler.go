package references

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Handler struct{store *Store;events *events.Store;provenance *provenance.Store;actions *humanactions.Store}

func NewHandler(store *Store,ev *events.Store,pv *provenance.Store,actions *humanactions.Store)(*Handler,error){if store==nil{return nil,errors.New("references handler requires store")};return &Handler{store:store,events:ev,provenance:pv,actions:actions},nil}

func (h *Handler) Register(mux *http.ServeMux){
	mux.HandleFunc("GET /api/v1/projects/{project_id}/references",h.list)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/references",h.create)
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}/references/{reference_id}",h.update)
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}/references/{reference_id}",h.delete)
}

func userID(r *http.Request)(uuid.UUID,bool){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{return uuid.Nil,false};return p.UserID,true}
func decode(r *http.Request,v any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")};return nil}
func writeJSON(w http.ResponseWriter,s int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(s);_ = json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,s int,c,m string){writeJSON(w,s,map[string]any{"error":map[string]string{"code":c,"message":m,"request_id":uuid.NewString()}})}

func (h *Handler) list(w http.ResponseWriter,r *http.Request){u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};items,err:=h.store.List(r.Context(),u,pid);if err!=nil{writeErr(w,500,"reference_list_failed","could not list references");return};writeJSON(w,200,map[string]any{"references":items})}
func (h *Handler) create(w http.ResponseWriter,r *http.Request){u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};var in Request;if err:=decode(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid reference payload");return};item,err:=h.store.Create(r.Context(),u,pid,in);if errors.Is(err,ErrInvalidReference){writeErr(w,400,"invalid_reference","reference payload is invalid");return};if errors.Is(err,ErrReferenceNotFound){writeErr(w,404,"project_not_found","project not found");return};if err!=nil{writeErr(w,500,"reference_create_failed","could not create reference");return};h.record(r,pid,item.ID,"reference.created",map[string]any{"source_type":item.SourceType,"license":item.License,"license_verified":item.LicenseVerified,"user_owned":item.UserOwned,"influence":item.Influence});writeJSON(w,201,item)}
func (h *Handler) update(w http.ResponseWriter,r *http.Request){u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,e1:=uuid.Parse(r.PathValue("project_id"));rid,e2:=uuid.Parse(r.PathValue("reference_id"));if e1!=nil||e2!=nil{writeErr(w,400,"invalid_reference_id","invalid reference id");return};var in Request;if err:=decode(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid reference payload");return};item,err:=h.store.Update(r.Context(),u,pid,rid,in);if errors.Is(err,ErrInvalidReference){writeErr(w,400,"invalid_reference","reference payload is invalid");return};if errors.Is(err,ErrReferenceNotFound){writeErr(w,404,"reference_not_found","reference not found");return};if err!=nil{writeErr(w,500,"reference_update_failed","could not update reference");return};h.record(r,pid,item.ID,"reference.updated",map[string]any{"license":item.License,"license_verified":item.LicenseVerified,"influence":item.Influence});writeJSON(w,200,item)}
func (h *Handler) delete(w http.ResponseWriter,r *http.Request){u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,e1:=uuid.Parse(r.PathValue("project_id"));rid,e2:=uuid.Parse(r.PathValue("reference_id"));if e1!=nil||e2!=nil{writeErr(w,400,"invalid_reference_id","invalid reference id");return};if err:=h.store.Delete(r.Context(),u,pid,rid);errors.Is(err,ErrReferenceNotFound){writeErr(w,404,"reference_not_found","reference not found");return}else if err!=nil{writeErr(w,500,"reference_delete_failed","could not delete reference");return};h.record(r,pid,rid,"reference.deleted",nil);w.WriteHeader(http.StatusNoContent)}
func(h *Handler)record(r *http.Request,pid,eid uuid.UUID,action string,payload map[string]any){u,_:=userID(r);if h.events!=nil{_,_=h.events.Append(r.Context(),u,pid,action,"reference",eid,payload)};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:pid,EntityType:"reference",EntityID:eid,Action:action,Payload:payload});if h.events!=nil{_,_=h.events.Append(r.Context(),u,pid,"provenance.updated","reference",eid,map[string]any{"action":action})}};if h.actions!=nil && action=="reference.created"{_,_=h.actions.Create(r.Context(),u,pid,humanactions.Request{ActionType:"REFERENCE_ADDED",Payload:payload,NewState:payload})};}
