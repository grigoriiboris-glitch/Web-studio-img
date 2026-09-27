
package prompts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Handler struct{ store *Store; events *events.Store; provenance *provenance.Store; actions *humanactions.Store }
func NewHandler(store *Store, ev *events.Store, pv *provenance.Store, actions *humanactions.Store)(*Handler,error){if store==nil{return nil,errors.New("prompt handler requires store")};return &Handler{store:store,events:ev,provenance:pv,actions:actions},nil}
func (h *Handler) Register(mux *http.ServeMux){
	mux.HandleFunc("GET /api/v1/projects/{project_id}/prompts",h.list)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/prompts",h.create)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/prompts/{prompt_id}",h.get)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/prompts/{prompt_id}/approve",h.approve)
}
func (h *Handler) create(w http.ResponseWriter,r *http.Request){
	u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return}
	var in struct{IterationID *uuid.UUID `json:"iteration_id,omitempty"`;ParentPromptID *uuid.UUID `json:"parent_prompt_id,omitempty"`;OriginalText string `json:"original_text"`;AISuggestions []string `json:"ai_suggestions,omitempty"`;FinalText *string `json:"final_text,omitempty"`;Components map[string]string `json:"components,omitempty"`;CreatedBy CreatedBy `json:"created_by,omitempty"`}
	if err:=decode(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid prompt payload");return};if in.CreatedBy==""{in.CreatedBy=CreatedByHuman}
	p,err:=h.store.Create(r.Context(),u,pid,Request{IterationID:in.IterationID,ParentPromptID:in.ParentPromptID,OriginalText:in.OriginalText,AISuggestions:in.AISuggestions,FinalText:in.FinalText,Components:in.Components,CreatedBy:in.CreatedBy})
	if errors.Is(err,ErrInvalidPrompt){writeErr(w,400,"invalid_prompt","prompt payload is invalid");return};if errors.Is(err,ErrPromptNotFound){writeErr(w,404,"prompt_parent_not_found","project or parent prompt not found");return};if err!=nil{writeErr(w,500,"prompt_create_failed","could not create prompt");return}
	h.record(r,pid,p.ID,in.IterationID,"prompt_created",map[string]any{"version":p.Version,"created_by":p.CreatedBy});writeJSON(w,201,p)
}
func (h *Handler) approve(w http.ResponseWriter,r *http.Request){
	u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};sid,err:=uuid.Parse(r.PathValue("prompt_id"));if err!=nil{writeErr(w,400,"invalid_prompt_id","invalid prompt id");return}
	source,err:=h.store.Get(r.Context(),u,sid);if errors.Is(err,ErrPromptNotFound)||source.ProjectID!=pid{writeErr(w,404,"prompt_not_found","prompt not found");return};if err!=nil{writeErr(w,500,"prompt_get_failed","could not load prompt");return}
	var in struct{FinalText string `json:"final_text"`;Components map[string]string `json:"components,omitempty"`};if err:=decode(r,&in);err!=nil||strings.TrimSpace(in.FinalText)==""{writeErr(w,400,"invalid_request","final_text is required");return}
	p,err:=h.store.Create(r.Context(),u,pid,Request{IterationID:source.IterationID,ParentPromptID:&source.ID,OriginalText:source.OriginalText,AISuggestions:source.AISuggestions,FinalText:&in.FinalText,Components:in.Components,CreatedBy:CreatedByHuman});if err!=nil{writeErr(w,500,"prompt_approve_failed","could not approve prompt");return}
	h.record(r,pid,p.ID,p.IterationID,"prompt_approved",map[string]any{"version":p.Version,"parent_prompt_id":source.ID});writeJSON(w,201,p)
}
func (h *Handler) list(w http.ResponseWriter,r *http.Request){u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};items,err:=h.store.List(r.Context(),u,pid);if err!=nil{writeErr(w,500,"prompt_list_failed","could not list prompts");return};writeJSON(w,200,map[string]any{"prompts":items})}
func (h *Handler) get(w http.ResponseWriter,r *http.Request){u,ok:=userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return};id,err:=uuid.Parse(r.PathValue("prompt_id"));if err!=nil{writeErr(w,400,"invalid_prompt_id","invalid prompt id");return};p,err:=h.store.Get(r.Context(),u,id);if errors.Is(err,ErrPromptNotFound){writeErr(w,404,"prompt_not_found","prompt not found");return};if err!=nil{writeErr(w,500,"prompt_get_failed","could not load prompt");return};writeJSON(w,200,p)}
func (h *Handler) record(r *http.Request,pid,eid uuid.UUID,iterationID *uuid.UUID,action string,payload map[string]any){
	u,_:=userID(r)
	if h.events!=nil{_,_=h.events.Append(r.Context(),u,pid,action,"prompt",eid,payload)}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:pid,IterationID:iterationID,EntityType:"prompt",EntityID:eid,Action:action,Payload:payload})}
	if h.actions!=nil{actionType:="PROMPT_EDITED";if action=="prompt_approved"{actionType="PROMPT_APPROVED"};_,_=h.actions.Create(r.Context(), u, pid, humanactions.Request{IterationID: iterationID, ActionType: actionType, Payload: payload, NewState: payload, AIInfluence: map[string]any{"source":"user"}})}
}
func decode(r *http.Request,v any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")};return nil}
func userID(r *http.Request)(uuid.UUID,bool){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{return uuid.Nil,false};return p.UserID,true}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,status int,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}
