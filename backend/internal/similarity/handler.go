package similarity

import (
 "context"
 "encoding/json"
 "errors"
 "io"
 "net/http"
 "github.com/google/uuid"
 "github.com/oleg3190/Web-studio-img/backend/internal/assets"
 "github.com/oleg3190/Web-studio-img/backend/internal/auth"
 "github.com/oleg3190/Web-studio-img/backend/internal/events"
 "github.com/oleg3190/Web-studio-img/backend/internal/provenance"
 "github.com/oleg3190/Web-studio-img/backend/internal/references"
 "github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Handler struct { refs *references.Store; assets *assets.Store; storage storage.StorageProvider; events *events.Store; provenance *provenance.Store }
func NewHandler(refs *references.Store, assetsStore *assets.Store, storageProvider storage.StorageProvider, ev *events.Store, pv *provenance.Store)(*Handler,error){if refs==nil||assetsStore==nil{return nil,errors.New("similarity handler requires stores")};return &Handler{refs:refs,assets:assetsStore,storage:storageProvider,events:ev,provenance:pv},nil}
func(h *Handler)Register(mux *http.ServeMux){mux.HandleFunc("POST /api/v1/projects/{project_id}/references/{reference_id}/influence",h.analyze)}
func(h *Handler)analyze(w http.ResponseWriter,r *http.Request){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return};if h.storage==nil{errJSON(w,503,"storage_unavailable","object storage is not configured");return};pid,e1:=uuid.Parse(r.PathValue("project_id"));rid,e2:=uuid.Parse(r.PathValue("reference_id"));if e1!=nil||e2!=nil{errJSON(w,400,"invalid_reference_id","invalid reference id");return};var in struct{TargetAssetID uuid.UUID `+tag+`json:"target_asset_id"`+tag+`};if err:=json.NewDecoder(io.LimitReader(r.Body,1<<20)).Decode(&in);err!=nil||in.TargetAssetID==uuid.Nil{errJSON(w,400,"invalid_request","target_asset_id is required");return};ref,err:=h.refs.GetOwned(r.Context(),p.UserID,pid,rid);if err!=nil||ref.AssetID==nil{errJSON(w,404,"reference_asset_not_found","reference must reference an asset");return};ra,err:=h.assets.GetOwned(r.Context(),p.UserID,pid,*ref.AssetID);if err!=nil{errJSON(w,404,"reference_asset_not_found","reference asset not found");return};ta,err:=h.assets.GetOwned(r.Context(),p.UserID,pid,in.TargetAssetID);if err!=nil{errJSON(w,404,"target_asset_not_found","target asset not found");return};rdata,err:=readObject(r.Context(),h.storage,ra.StorageKey);if err!=nil{errJSON(w,502,"reference_asset_read_failed","could not read reference asset");return};tdata,err:=readObject(r.Context(),h.storage,ta.StorageKey);if err!=nil{errJSON(w,502,"target_asset_read_failed","could not read target asset");return};scores,err:=Scores(rdata,tdata);if err!=nil{errJSON(w,400,"influence_analysis_failed","could not analyze image influence");return};inf:=&references.Influence{Composition:scores["composition"],Semantic:scores["semantic"],Color:scores["color"],Style:scores["style"],Material:scores["material"],Geometry:scores["geometry"]};if inf.Composition>=0.8{inf.Warning="High composition similarity; review before use."};updated,err:=h.refs.UpdateInfluence(r.Context(),p.UserID,pid,rid,inf);if err!=nil{errJSON(w,500,"influence_persist_failed","could not persist influence analysis");return};payload:=map[string]any{"target_asset_id":in.TargetAssetID,"influence":updated.Influence};if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,pid,"similarity.completed","reference",rid,payload)};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:p.UserID,ProjectID:pid,EntityType:"reference",EntityID:rid,Action:"reference.influence_analyzed",Payload:payload});if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,pid,"provenance.updated","reference",rid,map[string]any{"action":"reference.influence_analyzed"})}};writeJSON(w,200,updated)}
func readObject(ctx context.Context,s storage.StorageProvider,key string)([]byte,error){obj,_,err:=s.Get(ctx,key);if err!=nil{return nil,err};defer obj.Close();return io.ReadAll(io.LimitReader(obj,10<<20+1))}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func errJSON(w http.ResponseWriter,status,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}
