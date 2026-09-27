package exports

import (
 "encoding/json"
 "errors"
 "net/http"
 "time"
 "github.com/google/uuid"
 "github.com/oleg3190/Web-studio-img/backend/internal/assets"
 "github.com/oleg3190/Web-studio-img/backend/internal/auth"
 "github.com/oleg3190/Web-studio-img/backend/internal/events"
 "github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
 "github.com/oleg3190/Web-studio-img/backend/internal/provenance"
 "github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Handler struct{assets *assets.Store;storage storage.StorageProvider;events *events.Store;provenance *provenance.Store;actions *humanactions.Store}
func NewHandler(a *assets.Store,s storage.StorageProvider,e *events.Store,p *provenance.Store,h *humanactions.Store)(*Handler,error){if a==nil{return nil,errors.New("export handler requires asset store")};return &Handler{assets:a,storage:s,events:e,provenance:p,actions:h},nil}
func(h *Handler)Register(mux *http.ServeMux){mux.HandleFunc("POST /api/v1/projects/{project_id}/assets/{asset_id}/export",h.export)}
func(h *Handler)export(w http.ResponseWriter,r *http.Request){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return};if h.storage==nil{errJSON(w,503,"storage_unavailable","object storage is not configured");return};pid,e1:=uuid.Parse(r.PathValue("project_id"));id,e2:=uuid.Parse(r.PathValue("asset_id"));if e1!=nil||e2!=nil{errJSON(w,400,"invalid_asset_id","invalid asset id");return};asset,err:=h.assets.GetOwned(r.Context(),p.UserID,pid,id);if err!=nil{errJSON(w,404,"asset_not_found","asset not found");return};url,err:=h.storage.PresignGet(r.Context(),asset.StorageKey,10*time.Minute);if err!=nil{errJSON(w,503,"export_failed","could not create export URL");return};payload:=map[string]any{"asset_id":id,"url":url.URL,"expires_at":url.ExpiresAt};if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,pid,"export.completed","asset",id,payload)};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:p.UserID,ProjectID:pid,EntityType:"export",EntityID:id,Action:"export.completed",Payload:payload});if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,pid,"provenance.updated","asset",id,map[string]any{"action":"export.completed"})}};if h.actions!=nil{_,_=h.actions.Create(r.Context(),p.UserID,pid,humanactions.Request{ActionType:"EXPORT_CREATED",Payload:payload,NewState:payload})};writeJSON(w,200,payload)}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func errJSON(w http.ResponseWriter,status,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}
