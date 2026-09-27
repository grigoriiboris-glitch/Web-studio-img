package composition

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Analyzer struct {
	db *sql.DB
	assets *assets.Store
	storage storage.StorageProvider
	events *events.Store
	provenance *provenance.Store
}

func NewAnalyzer(db *sql.DB, assetsStore *assets.Store, storageProvider storage.StorageProvider, ev *events.Store, pv *provenance.Store) (*Analyzer,error) {
	if db==nil||assetsStore==nil { return nil,errors.New("composition analyzer requires database and asset store") }
	return &Analyzer{db:db,assets:assetsStore,storage:storageProvider,events:ev,provenance:pv},nil
}
func(a *Analyzer) Register(mux *http.ServeMux){mux.HandleFunc("POST /api/v1/projects/{project_id}/iterations/{iteration_id}/composition/analyze",a.analyze)}
func(a *Analyzer) analyze(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.PrincipalFromContext(r.Context());if !ok{returnErr(w,401,"unauthorized","authentication required");return}
	if a.storage==nil{returnErr(w,503,"composition_unavailable","object storage is not configured");return}
	pid,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil{returnErr(w,400,"invalid_project_id","invalid project id");return}
	iid,e:=uuid.Parse(r.PathValue("iteration_id"));if e!=nil{returnErr(w,400,"invalid_iteration_id","invalid iteration id");return}
	var in struct{AssetID uuid.UUID `json:"asset_id"`}
	d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if e=d.Decode(&in);e!=nil||in.AssetID==uuid.Nil{returnErr(w,400,"invalid_request","asset_id is required");return}
	var extra any;if e=d.Decode(&extra);e!=io.EOF{returnErr(w,400,"invalid_request","request must contain one JSON object");return}
	var owned bool
	e=a.db.QueryRowContext(r.Context(),`SELECT EXISTS(SELECT 1 FROM iterations i JOIN projects p ON p.id=i.project_id WHERE i.id=$1 AND i.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted')`,iid,pid,p.UserID).Scan(&owned);if e!=nil||!owned{returnErr(w,404,"iteration_not_found","iteration not found");return}
	asset,e:=a.assets.GetOwned(r.Context(),p.UserID,pid,in.AssetID);if e!=nil{returnErr(w,404,"asset_not_found","asset not found");return}
	obj,_,e:=a.storage.Get(r.Context(),asset.StorageKey);if e!=nil{returnErr(w,502,"asset_read_failed","could not read asset");return};defer obj.Close()
	data,e:=io.ReadAll(io.LimitReader(obj,assets.MaxAssetSize+1));if e!=nil{returnErr(w,502,"asset_read_failed","could not read asset");return};if int64(len(data))>assets.MaxAssetSize{returnErr(w,400,"asset_too_large","asset exceeds analysis limit");return}
	desc,e:=similarity.AnalyzeComposition(data);if e!=nil{returnErr(w,400,"composition_analysis_failed","could not analyze composition");return}
	raw:=func(key string)[]byte{b,_:=json.Marshal(desc[key]);if len(b)==0||string(b)=="null"{return []byte("{}")};return b}
	var spec Spec;var fp,bb,rp,hi,ns,dg,os,ld []byte
	e=a.db.QueryRowContext(r.Context(),`INSERT INTO composition_specs(project_id,iteration_id,user_id,focal_points,bounding_boxes,relative_positions,horizon,camera_elevation,perspective,hierarchy,negative_space,dominant_geometry,object_scale,light_direction) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(iteration_id) DO UPDATE SET focal_points=excluded.focal_points,bounding_boxes=excluded.bounding_boxes,relative_positions=excluded.relative_positions,horizon=excluded.horizon,camera_elevation=excluded.camera_elevation,perspective=excluded.perspective,hierarchy=excluded.hierarchy,negative_space=excluded.negative_space,dominant_geometry=excluded.dominant_geometry,object_scale=excluded.object_scale,light_direction=excluded.light_direction,updated_at=now() RETURNING id,project_id,iteration_id,user_id,focal_points,bounding_boxes,relative_positions,horizon,camera_elevation,perspective,hierarchy,negative_space,dominant_geometry,object_scale,light_direction,created_at,updated_at`,pid,iid,p.UserID,raw("focal_points"),raw("bounding_boxes"),raw("relative_positions"),desc["horizon"],desc["camera_elevation"],stringValue(desc["perspective"]),raw("hierarchy"),raw("negative_space"),raw("dominant_geometry"),raw("object_scale"),raw("light_direction")).Scan(&spec.ID,&spec.ProjectID,&spec.IterationID,&spec.UserID,&fp,&bb,&rp,&spec.Horizon,&spec.CameraElevation,&spec.Perspective,&hi,&ns,&dg,&os,&ld,&spec.CreatedAt,&spec.UpdatedAt)
	if e!=nil{returnErr(w,500,"composition_save_failed","could not persist composition analysis");return}
	_=json.Unmarshal(fp,&spec.FocalPoints);_=json.Unmarshal(bb,&spec.BoundingBoxes);_=json.Unmarshal(rp,&spec.RelativePositions);_=json.Unmarshal(hi,&spec.Hierarchy);_=json.Unmarshal(ns,&spec.NegativeSpace);_=json.Unmarshal(dg,&spec.DominantGeometry);_=json.Unmarshal(os,&spec.ObjectScale);_=json.Unmarshal(ld,&spec.LightDirection)
	payload:=map[string]any{"iteration_id":iid,"composition_spec_id":spec.ID,"asset_id":asset.ID,"analysis_mode":desc["analysis_mode"]}
	if a.events!=nil{_,_=a.events.Append(r.Context(),p.UserID,pid,"composition.completed","composition",spec.ID,payload)}
	if a.provenance!=nil{_,_=a.provenance.Append(r.Context(),provenance.Event{UserID:p.UserID,ProjectID:pid,IterationID:&iid,EntityType:"composition",EntityID:spec.ID,Action:"composition.analyzed",Payload:payload})}
	writeJSON(w,200,map[string]any{"spec":spec,"analysis":desc,"uncertainty":"Deterministic image descriptors only; no learned object detection or segmentation."})
}
func returnErr(w http.ResponseWriter,status int,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}
