package variants

import (
  "context"
  "database/sql"
  "encoding/json"
  "errors"
  "io"
  "net/http"
  "strconv"
  "sort"
  "strings"
  "time"

  "github.com/google/uuid"

  "github.com/oleg3190/Web-studio-img/backend/internal/auth"
  "github.com/oleg3190/Web-studio-img/backend/internal/events"
  "github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
  "github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

var ErrNotFound = errors.New("variant board resource not found")

var allowedRejectReasons = map[string]bool{"composition":true,"subject":true,"pose":true,"lighting":true,"color":true,"material":true,"background":true,"object":true,"style":true,"prompt":true,"quality":true,"other":true}
var allowedRejectSeverities = map[string]bool{"low":true,"medium":true,"high":true}

type Handler struct {
  db *sql.DB
  actions *humanactions.Store
  provenance *provenance.Store
  events *events.Store
}

type VariantSet struct {
  ID uuid.UUID `json:"id"`
  ProjectID uuid.UUID `json:"project_id"`
  UserID uuid.UUID `json:"user_id"`
  Name string `json:"name"`
  Metadata map[string]any `json:"metadata"`
  CreatedAt time.Time `json:"created_at"`
}

type VariantSource struct {
  GenerationID uuid.UUID `json:"generation_id"`
  IterationID *uuid.UUID `json:"iteration_id,omitempty"`
  AssetID *uuid.UUID `json:"asset_id,omitempty"`
  Status string `json:"status"`
  Prompt string `json:"prompt"`
  Provider string `json:"provider"`
  Model string `json:"model"`
  CreatedAt time.Time `json:"created_at"`
  Width *int `json:"width,omitempty"`
  Height *int `json:"height,omitempty"`
}

type Variant struct {
  ID uuid.UUID `json:"id"`
  VariantSetID uuid.UUID `json:"variant_set_id"`
  GenerationID uuid.UUID `json:"generation_id"`
  AssetID *uuid.UUID `json:"asset_id,omitempty"`
  Ordinal int `json:"ordinal"`
  Decision string `json:"decision"`
  Favorite bool `json:"favorite"`
  CompareSelected bool `json:"compare_selected"`
  RejectReason []string `json:"reject_reason"`
  RejectComment *string `json:"reject_comment,omitempty"`
  RejectSeverity *string `json:"reject_severity,omitempty"`
  RejectReasonSkipped bool `json:"reject_reason_skipped"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
  Source *VariantSource `json:"source,omitempty"`
}

type CreateSetRequest struct {
  Name string `json:"name"`
  GenerationIDs []uuid.UUID `json:"generation_ids"`
}

type VariantPatch struct {
  Decision *string `json:"decision,omitempty"`
  Favorite *bool `json:"favorite,omitempty"`
  CompareSelected *bool `json:"compare_selected,omitempty"`
  RejectReason []string `json:"reject_reason,omitempty"`
  RejectComment *string `json:"reject_comment,omitempty"`
  RejectSeverity *string `json:"reject_severity,omitempty"`
  SkipReason *bool `json:"skip_reason,omitempty"`
}

func NewHandler(db *sql.DB, actions *humanactions.Store, provenanceStore *provenance.Store, eventStore *events.Store) (*Handler, error) {
  if db == nil { return nil, errors.New("variant handler requires database") }
  return &Handler{db: db, actions: actions, provenance: provenanceStore, events: eventStore}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
  mux.HandleFunc("GET /api/v1/projects/{project_id}/variant-sources", h.sources)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/variant-sets", h.listSets)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/variant-sets", h.createSet)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/variant-sets/{set_id}", h.getSet)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/variant-sets/{set_id}/compare", h.compare)
  mux.HandleFunc("PATCH /api/v1/projects/{project_id}/variant-sets/{set_id}/variants/{variant_id}", h.patchVariant)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/variant-sets/{set_id}/iterations", h.createIteration)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/variant-rejection-summary", h.rejectionSummary)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/variant-rejection-timeline", h.rejectionTimeline)
}

func (h *Handler) sources(w http.ResponseWriter, r *http.Request) {
  userID, projectID, ok := h.authProject(w, r); if !ok { return }
  rows, err := h.db.QueryContext(r.Context(), `
    SELECT g.id,g.iteration_id,g.status,g.prompt,g.provider,g.model,g.created_at,
           a.id,a.width,a.height
    FROM generations g
    LEFT JOIN assets a ON a.generation_id=g.id AND a.user_id=$2 AND a.lifecycle_status='active'
    WHERE g.project_id=$1 AND g.user_id=$2
    ORDER BY g.created_at DESC LIMIT 100
  `, projectID, userID)
  if err != nil { writeError(w,500,"variant_sources_failed","could not list generation sources"); return }
  defer func(){ _ = rows.Close() }()
  out := make([]VariantSource,0)
  for rows.Next() {
    var x VariantSource
    var iteration, asset sql.NullString
    var width,height sql.NullInt64
    if err := rows.Scan(&x.GenerationID,&iteration,&x.Status,&x.Prompt,&x.Provider,&x.Model,&x.CreatedAt,&asset,&width,&height); err != nil {
      writeError(w,500,"variant_sources_failed","could not read generation source"); return
    }
    if iteration.Valid { if id,e:=uuid.Parse(iteration.String); e==nil { x.IterationID=&id } }
    if asset.Valid { if id,e:=uuid.Parse(asset.String); e==nil { x.AssetID=&id } }
    if width.Valid { v:=int(width.Int64); x.Width=&v }
    if height.Valid { v:=int(height.Int64); x.Height=&v }
    out=append(out,x)
  }
  if err:=rows.Err();err!=nil { writeError(w,500,"variant_sources_failed","could not read generation sources"); return }
  writeJSON(w,200,map[string]any{"sources":out})
}

func (h *Handler) listSets(w http.ResponseWriter, r *http.Request) {
  userID,projectID,ok:=h.authProject(w,r); if !ok{return}
  rows,err:=h.db.QueryContext(r.Context(), `
    SELECT id,project_id,user_id,name,metadata,created_at
    FROM variant_sets WHERE project_id=$1 AND user_id=$2
    ORDER BY created_at DESC LIMIT 100
  `,projectID,userID)
  if err!=nil { writeError(w,500,"variant_set_list_failed","could not list variant sets"); return }
  defer func(){ _=rows.Close() }()
  out:=make([]VariantSet,0)
  for rows.Next(){
    var x VariantSet; var raw []byte
    if err:=rows.Scan(&x.ID,&x.ProjectID,&x.UserID,&x.Name,&raw,&x.CreatedAt);err!=nil{writeError(w,500,"variant_set_list_failed","could not read variant set");return}
    _=json.Unmarshal(raw,&x.Metadata);if x.Metadata==nil{x.Metadata=map[string]any{}}
    out=append(out,x)
  }
  if err:=rows.Err();err!=nil{writeError(w,500,"variant_set_list_failed","could not read variant sets");return}
  writeJSON(w,200,map[string]any{"variant_sets":out})
}

func (h *Handler) createSet(w http.ResponseWriter, r *http.Request) {
  userID,projectID,ok:=h.authProject(w,r);if !ok{return}
  var in CreateSetRequest
  if err:=decode(r,&in);err!=nil || strings.TrimSpace(in.Name)=="" || len([]rune(in.Name))>200 || len(in.GenerationIDs)<2 || len(in.GenerationIDs)>50{
    writeError(w,400,"invalid_variant_set","name and 2..50 generation_ids are required");return
  }
  seen:=map[uuid.UUID]bool{}
  for _,id:=range in.GenerationIDs{if id==uuid.Nil||seen[id]{writeError(w,400,"invalid_variant_set","generation_ids must be unique valid UUIDs");return};seen[id]=true}
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"variant_set_create_failed","could not start transaction");return}
  defer func(){_=tx.Rollback()}()
  var set VariantSet;var metadata []byte
  err=tx.QueryRowContext(r.Context(), `
    INSERT INTO variant_sets(project_id,user_id,name,metadata) VALUES($1,$2,$3,'{}'::jsonb)
    RETURNING id,project_id,user_id,name,metadata,created_at
  `,projectID,userID,strings.TrimSpace(in.Name)).Scan(&set.ID,&set.ProjectID,&set.UserID,&set.Name,&metadata,&set.CreatedAt)
  if err!=nil{writeError(w,500,"variant_set_create_failed","could not create variant set");return}
  _=json.Unmarshal(metadata,&set.Metadata);if set.Metadata==nil{set.Metadata=map[string]any{}}
  variants:=make([]Variant,0,len(in.GenerationIDs))
  for i,generationID:=range in.GenerationIDs{
    var v Variant;var assetID sql.NullString
    err=tx.QueryRowContext(r.Context(), `
      SELECT g.id,a.id FROM generations g
      LEFT JOIN assets a ON a.generation_id=g.id AND a.user_id=$2 AND a.lifecycle_status='active'
      WHERE g.id=$1 AND g.project_id=$3 AND g.user_id=$2 AND g.status='succeeded' LIMIT 1
    `,generationID,userID,projectID).Scan(&v.GenerationID,&assetID)
    if errors.Is(err,sql.ErrNoRows){writeError(w,400,"invalid_variant_source","all generations must be owned, succeeded and belong to this project");return}
    if err!=nil{writeError(w,500,"variant_set_create_failed","could not validate generation source");return}
    if assetID.Valid{if id,e:=uuid.Parse(assetID.String);e==nil{v.AssetID=&id}}
    v.ID=uuid.New();v.VariantSetID=set.ID;v.Ordinal=i+1;v.Decision="candidate";v.RejectReason=[]string{}
    if _,err=tx.ExecContext(r.Context(), `
      INSERT INTO variants(id,variant_set_id,project_id,user_id,generation_id,asset_id,ordinal,reject_reason)
      VALUES($1,$2,$3,$4,$5,$6,$7,'[]'::jsonb)
    `,v.ID,set.ID,projectID,userID,v.GenerationID,v.AssetID,v.Ordinal);err!=nil{writeError(w,500,"variant_set_create_failed","could not create variant");return}
    variants=append(variants,v)
  }
  if err:=tx.Commit();err!=nil{writeError(w,500,"variant_set_create_failed","could not commit variant set");return}
  h.audit(r.Context(),userID,projectID,set.ID,"VARIANT_SET_CREATED",map[string]any{"variant_set_id":set.ID,"generation_ids":in.GenerationIDs,"variant_count":len(variants)})
  writeJSON(w,201,map[string]any{"variant_set":set,"variants":variants})
}

func (h *Handler) getSet(w http.ResponseWriter,r *http.Request){
  userID,projectID,ok:=h.authProject(w,r);if !ok{return}
  setID,err:=uuid.Parse(r.PathValue("set_id"));if err!=nil{writeError(w,400,"invalid_variant_set_id","invalid variant set id");return}
  set,err:=h.loadSet(r.Context(),userID,projectID,setID);if errors.Is(err,ErrNotFound){writeError(w,404,"variant_set_not_found","variant set not found");return};if err!=nil{writeError(w,500,"variant_set_get_failed","could not load variant set");return}
  variants,err:=h.loadVariants(r.Context(),userID,projectID,setID,"");if err!=nil{writeError(w,500,"variant_list_failed","could not list variants");return}
  writeJSON(w,200,map[string]any{"variant_set":set,"variants":variants})
}

func (h *Handler) compare(w http.ResponseWriter,r *http.Request){
  userID,projectID,ok:=h.authProject(w,r);if !ok{return}
  setID,err:=uuid.Parse(r.PathValue("set_id"));if err!=nil{writeError(w,400,"invalid_variant_set_id","invalid variant set id");return}
  if _,err=h.loadSet(r.Context(),userID,projectID,setID);errors.Is(err,ErrNotFound){writeError(w,404,"variant_set_not_found","variant set not found");return}
  ids:=r.URL.Query()["variant_id"];if len(ids)<2||len(ids)>4{writeError(w,400,"invalid_compare","compare requires 2..4 variant_id values");return}
  variants,err:=h.loadVariants(r.Context(),userID,projectID,setID,strings.Join(ids,","));if err!=nil{writeError(w,500,"variant_compare_failed","could not load compare variants");return}
  if len(variants)!=len(ids){writeError(w,404,"variant_not_found","one or more variants not found");return}
  writeJSON(w,200,map[string]any{"variants":variants})
}

func (h *Handler) patchVariant(w http.ResponseWriter, r *http.Request) {
  userID, projectID, ok := h.authProject(w, r)
  if !ok { return }
  setID, err := uuid.Parse(r.PathValue("set_id"))
  if err != nil { writeError(w, 400, "invalid_variant_set_id", "invalid variant set id"); return }
  variantID, err := uuid.Parse(r.PathValue("variant_id"))
  if err != nil { writeError(w, 400, "invalid_variant_id", "invalid variant id"); return }
  var in VariantPatch
  if err := decode(r, &in); err != nil { writeError(w, 400, "invalid_variant_patch", "invalid variant update"); return }
  if in.Decision != nil && !map[string]bool{"candidate":true,"kept":true,"rejected":true,"selected":true}[*in.Decision] { writeError(w,400,"invalid_variant_decision","invalid variant decision"); return }
  if in.RejectReason != nil {
    if len(in.RejectReason) > 12 { writeError(w,400,"invalid_reject_reason","at most 12 reject reasons are allowed"); return }
    normalized := make([]string,0,len(in.RejectReason)); seen := map[string]bool{}
    for _, reason := range in.RejectReason {
      value := strings.ToLower(strings.TrimSpace(reason))
      if !allowedRejectReasons[value] || seen[value] { writeError(w,400,"invalid_reject_reason","unknown or duplicated reject reason"); return }
      seen[value] = true; normalized = append(normalized,value)
    }
    sort.Strings(normalized); in.RejectReason = normalized
  }
  if in.RejectComment != nil && len([]rune(*in.RejectComment)) > 2000 { writeError(w,400,"invalid_reject_comment","reject comment is limited to 2000 characters"); return }
  if in.RejectSeverity != nil {
    value := strings.ToLower(strings.TrimSpace(*in.RejectSeverity));
    if !allowedRejectSeverities[value] { writeError(w,400,"invalid_reject_severity","severity must be low, medium or high"); return };
    in.RejectSeverity = &value
  }
  current, err := h.loadVariant(r.Context(),userID,projectID,setID,variantID)
  if errors.Is(err,ErrNotFound) { writeError(w,404,"variant_not_found","variant not found"); return }
  if err != nil { writeError(w,500,"variant_get_failed","could not load variant"); return }
  nextDecision,nextFavorite,nextCompare := current.Decision,current.Favorite,current.CompareSelected
  nextReasons := append([]string(nil),current.RejectReason...)
  nextComment,nextSeverity,nextSkipped := current.RejectComment,current.RejectSeverity,current.RejectReasonSkipped
  if in.Decision != nil { nextDecision = *in.Decision }
  if in.Favorite != nil { nextFavorite = *in.Favorite }
  if in.CompareSelected != nil { nextCompare = *in.CompareSelected }
  if in.RejectReason != nil { nextReasons = append([]string(nil),in.RejectReason...); nextSkipped = false }
  if in.RejectComment != nil { value:=strings.TrimSpace(*in.RejectComment); if value=="" { nextComment=nil } else { nextComment=&value } }
  if in.RejectSeverity != nil { nextSeverity=in.RejectSeverity }
  if in.SkipReason != nil { nextSkipped=*in.SkipReason; if *in.SkipReason { nextReasons=[]string{} } }
  if nextDecision=="rejected" {
    if !nextSkipped && len(nextReasons)==0 { writeError(w,400,"reject_reason_required","choose at least one reject reason or explicitly skip reason"); return }
  } else {
    nextReasons=[]string{}; nextComment=nil; nextSeverity=nil; nextSkipped=false
  }
  reasons,_:=json.Marshal(nextReasons)
  var comment any; if nextComment!=nil { comment=*nextComment }
  var severity any; if nextSeverity!=nil { severity=*nextSeverity }
  var v Variant; var raw []byte
  err=h.db.QueryRowContext(r.Context(), `UPDATE variants
    SET decision=$1,favorite=$2,compare_selected=$3,reject_reason=$4,reject_comment=$5,reject_severity=$6,reject_reason_skipped=$7,updated_at=now()
    WHERE id=$8 AND variant_set_id=$9 AND project_id=$10 AND user_id=$11
    RETURNING id,variant_set_id,generation_id,asset_id,ordinal,decision,favorite,compare_selected,reject_reason,reject_comment,reject_severity,reject_reason_skipped,created_at,updated_at`,
    nextDecision,nextFavorite,nextCompare,reasons,comment,severity,nextSkipped,variantID,setID,projectID,userID).Scan(
    &v.ID,&v.VariantSetID,&v.GenerationID,&v.AssetID,&v.Ordinal,&v.Decision,&v.Favorite,&v.CompareSelected,&raw,&v.RejectComment,&v.RejectSeverity,&v.RejectReasonSkipped,&v.CreatedAt,&v.UpdatedAt)
  if err!=nil { writeError(w,500,"variant_update_failed","could not update variant"); return }
  _=json.Unmarshal(raw,&v.RejectReason); if v.RejectReason==nil { v.RejectReason=[]string{} }
  if in.Decision!=nil || in.RejectReason!=nil || in.RejectComment!=nil || in.RejectSeverity!=nil || in.SkipReason!=nil {
    action:="VARIANT_SELECTED"; if v.Decision=="rejected" { action="VARIANT_REJECTED" }
    oldState,newState:=variantDecisionState(current),variantDecisionState(v)
    h.audit(r.Context(),userID,projectID,v.ID,action,map[string]any{"variant_id":v.ID,"variant_set_id":setID,"decision":v.Decision,"reject_reason":v.RejectReason,"reject_comment":v.RejectComment,"reject_severity":v.RejectSeverity,"reject_reason_skipped":v.RejectReasonSkipped},oldState,newState)
  }
  if in.Favorite!=nil { action:="VARIANT_UNFAVORITED"; if nextFavorite { action="VARIANT_FAVORITED" }; h.audit(r.Context(),userID,projectID,v.ID,action,map[string]any{"variant_id":v.ID,"variant_set_id":setID,"favorite":nextFavorite}) }
  writeJSON(w,200,v)
}
func (h *Handler) createIteration(w http.ResponseWriter,r *http.Request){
  userID,projectID,ok:=h.authProject(w,r);if !ok{return}
  setID,err:=uuid.Parse(r.PathValue("set_id"));if err!=nil{writeError(w,400,"invalid_variant_set_id","invalid variant set id");return}
  var in struct{VariantIDs []uuid.UUID `json:"variant_ids"`;Title string `json:"title"`}
  if err:=decode(r,&in);err!=nil||len(in.VariantIDs)<1||len(in.VariantIDs)>50{writeError(w,400,"invalid_selection","variant_ids must contain 1..50 values");return}
  seen:=map[uuid.UUID]bool{};for _,id:=range in.VariantIDs{if id==uuid.Nil||seen[id]{writeError(w,400,"invalid_selection","variant_ids must be unique valid UUIDs");return};seen[id]=true}
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{writeError(w,500,"iteration_create_failed","could not start transaction");return};defer func(){_=tx.Rollback()}()
  var parent sql.NullString;selectedJSON:=[]byte{}
  placeholders:=make([]string,0,len(in.VariantIDs));args:=[]any{projectID,userID,setID}
  for i,id:=range in.VariantIDs{placeholders=append(placeholders,"$"+strconv.Itoa(i+4));args=append(args,id)}
  query:=`SELECT MIN(g.iteration_id::text), COALESCE(json_agg(v.id::text ORDER BY v.ordinal),'[]'::json) FROM variants v JOIN generations g ON g.id=v.generation_id WHERE v.project_id=$1 AND v.user_id=$2 AND v.variant_set_id=$3 AND v.id IN (`+strings.Join(placeholders,",")+`)`
  if err:=tx.QueryRowContext(r.Context(),query,args...).Scan(&parent,&selectedJSON);err!=nil{writeError(w,500,"iteration_create_failed","could not load selected variants");return}
  var selected []string;_=json.Unmarshal(selectedJSON,&selected);if len(selected)!=len(in.VariantIDs){writeError(w,404,"variant_not_found","one or more variants not found");return}
  title:=strings.TrimSpace(in.Title);if title==""{title="Selection from Variant Board"}
  description:="Created from variant set "+setID.String()+"; selected variants: "+strings.Join(selected,", ")
  var iterationID uuid.UUID
  if err:=tx.QueryRowContext(r.Context(),`INSERT INTO iterations(project_id,parent_iteration_id,type,title,description) VALUES($1,CASE WHEN $2='' THEN NULL ELSE $2::uuid END,'selection',$3,$4) RETURNING id`,projectID,parent.String,title,description).Scan(&iterationID);err!=nil{writeError(w,500,"iteration_create_failed","could not create selection iteration");return}
  if err:=tx.Commit();err!=nil{writeError(w,500,"iteration_create_failed","could not commit selection iteration");return}
  h.audit(r.Context(),userID,projectID,iterationID,"VARIANT_SELECTED",map[string]any{"variant_set_id":setID,"variant_ids":in.VariantIDs,"created_iteration_id":iterationID})
  writeJSON(w,201,map[string]any{"iteration_id":iterationID,"title":title,"description":description})
}

func (h *Handler) loadSet(ctx context.Context,userID,projectID,setID uuid.UUID)(VariantSet,error){
  var x VariantSet;var raw []byte
  err:=h.db.QueryRowContext(ctx,`SELECT id,project_id,user_id,name,metadata,created_at FROM variant_sets WHERE id=$1 AND project_id=$2 AND user_id=$3`,setID,projectID,userID).Scan(&x.ID,&x.ProjectID,&x.UserID,&x.Name,&raw,&x.CreatedAt)
  if errors.Is(err,sql.ErrNoRows){return VariantSet{},ErrNotFound};if err!=nil{return VariantSet{},err};_=json.Unmarshal(raw,&x.Metadata);if x.Metadata==nil{x.Metadata=map[string]any{}}
  return x,nil
}

func (h *Handler) loadVariant(ctx context.Context,userID,projectID,setID,variantID uuid.UUID)(Variant,error){
  items,err:=h.loadVariants(ctx,userID,projectID,setID,variantID.String());if err!=nil{return Variant{},err};if len(items)==0{return Variant{},ErrNotFound};return items[0],nil
}

func (h *Handler) loadVariants(ctx context.Context,userID,projectID,setID uuid.UUID,filter string)([]Variant,error){
  args:=[]any{setID,projectID,userID};where:=""
  if filter!=""{ids:=strings.Split(filter,",");ph:=make([]string,0,len(ids));for i,id:=range ids{parsed,e:=uuid.Parse(strings.TrimSpace(id));if e!=nil{return nil,e};ph=append(ph,"$"+strconv.Itoa(i+4));args=append(args,parsed)};where=" AND v.id IN ("+strings.Join(ph,",")+")"}
  rows,err:=h.db.QueryContext(ctx,`
    SELECT v.id,v.variant_set_id,v.generation_id,v.asset_id,v.ordinal,v.decision,v.favorite,v.compare_selected,v.reject_reason,v.reject_comment,v.reject_severity,v.reject_reason_skipped,v.created_at,v.updated_at,
           g.iteration_id,g.status,g.prompt,g.provider,g.model,g.created_at,a.width,a.height
    FROM variants v JOIN generations g ON g.id=v.generation_id
    LEFT JOIN assets a ON a.id=v.asset_id AND a.user_id=$3 AND a.lifecycle_status='active'
    WHERE v.variant_set_id=$1 AND v.project_id=$2 AND v.user_id=$3`+where+` ORDER BY v.ordinal ASC`,args...)
  if err!=nil{return nil,err};defer func(){_=rows.Close()}()
  out:=make([]Variant,0)
  for rows.Next(){var v Variant;var raw []byte;var source VariantSource;var iteration sql.NullString;var width,height sql.NullInt64
    if err:=rows.Scan(&v.ID,&v.VariantSetID,&v.GenerationID,&v.AssetID,&v.Ordinal,&v.Decision,&v.Favorite,&v.CompareSelected,&raw,&v.RejectComment,&v.RejectSeverity,&v.RejectReasonSkipped,&v.CreatedAt,&v.UpdatedAt,&iteration,&source.Status,&source.Prompt,&source.Provider,&source.Model,&source.CreatedAt,&width,&height);err!=nil{return nil,err}
    _=json.Unmarshal(raw,&v.RejectReason);if v.RejectReason==nil{v.RejectReason=[]string{}};if iteration.Valid{if id,e:=uuid.Parse(iteration.String);e==nil{source.IterationID=&id}};source.GenerationID=v.GenerationID;source.AssetID=v.AssetID;if width.Valid{v1:=int(width.Int64);source.Width=&v1};if height.Valid{v1:=int(height.Int64);source.Height=&v1};v.Source=&source;out=append(out,v)
  }
  return out,rows.Err()
}

func variantDecisionState(v Variant) map[string]any {
  return map[string]any{"decision":v.Decision,"reject_reason":append([]string(nil),v.RejectReason...),"reject_comment":v.RejectComment,"reject_severity":v.RejectSeverity,"reject_reason_skipped":v.RejectReasonSkipped}
}

func (h *Handler) rejectionSummary(w http.ResponseWriter, r *http.Request) {
  userID,projectID,ok:=h.authProject(w,r); if !ok { return }
  var rejectedCount int
  if err:=h.db.QueryRowContext(r.Context(),`SELECT count(*) FROM variants WHERE project_id=$1 AND user_id=$2 AND decision='rejected'`,projectID,userID).Scan(&rejectedCount);err!=nil{writeError(w,500,"variant_rejection_summary_failed","could not calculate rejected count");return}
  rows,err:=h.db.QueryContext(r.Context(),`SELECT reason,count(*) FROM variants v CROSS JOIN LATERAL jsonb_array_elements_text(v.reject_reason) AS reason WHERE v.project_id=$1 AND v.user_id=$2 AND v.decision='rejected' GROUP BY reason ORDER BY count(*) DESC,reason ASC`,projectID,userID)
  if err!=nil{writeError(w,500,"variant_rejection_summary_failed","could not calculate rejection reasons");return};defer func(){_=rows.Close()}()
  type item struct{Reason string `json:"reason"`;Count int `json:"count"`;Percent float64 `json:"percent"`}
  reasons:=make([]item,0)
  for rows.Next(){var reason string;var count int;if err:=rows.Scan(&reason,&count);err!=nil{writeError(w,500,"variant_rejection_summary_failed","could not read rejection summary");return};percent:=0.0;if rejectedCount>0{percent=float64(count)*100/float64(rejectedCount)};reasons=append(reasons,item{reason,count,percent})}
  if err:=rows.Err();err!=nil{writeError(w,500,"variant_rejection_summary_failed","could not read rejection summary");return}
  writeJSON(w,200,map[string]any{"rejected_count":rejectedCount,"reasons":reasons})
}

func (h *Handler) rejectionTimeline(w http.ResponseWriter, r *http.Request) {
  userID,projectID,ok:=h.authProject(w,r); if !ok { return }
  rows,err:=h.db.QueryContext(r.Context(),`SELECT id,version,action_type,payload,old_state,new_state,created_at FROM human_actions WHERE project_id=$1 AND user_id=$2 AND action_type='VARIANT_REJECTED' ORDER BY version DESC LIMIT 100`,projectID,userID)
  if err!=nil{writeError(w,500,"variant_rejection_timeline_failed","could not load rejection timeline");return};defer func(){_=rows.Close()}()
  type item struct{ID uuid.UUID `json:"id"`;Version int64 `json:"version"`;ActionType string `json:"action_type"`;Payload map[string]any `json:"payload,omitempty"`;OldState map[string]any `json:"old_state,omitempty"`;NewState map[string]any `json:"new_state,omitempty"`;CreatedAt time.Time `json:"created_at"`}
  out:=make([]item,0)
  for rows.Next(){var x item;var payload,oldState,newState []byte;if err:=rows.Scan(&x.ID,&x.Version,&x.ActionType,&payload,&oldState,&newState,&x.CreatedAt);err!=nil{writeError(w,500,"variant_rejection_timeline_failed","could not read rejection timeline");return};_=json.Unmarshal(payload,&x.Payload);_=json.Unmarshal(oldState,&x.OldState);_=json.Unmarshal(newState,&x.NewState);out=append(out,x)}
  if err:=rows.Err();err!=nil{writeError(w,500,"variant_rejection_timeline_failed","could not read rejection timeline");return}
  writeJSON(w,200,map[string]any{"items":out})
}

func (h *Handler) authProject(w http.ResponseWriter,r *http.Request)(uuid.UUID,uuid.UUID,bool){
  p,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeError(w,401,"unauthorized","authentication required");return uuid.Nil,uuid.Nil,false}
  pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeError(w,400,"invalid_project_id","invalid project id");return uuid.Nil,uuid.Nil,false}
  var exists bool
  if err:=h.db.QueryRowContext(r.Context(),"SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')",pid,p.UserID).Scan(&exists);err!=nil||!exists{writeError(w,404,"project_not_found","project not found");return uuid.Nil,uuid.Nil,false}
  return p.UserID,pid,true
}

func decode(r *http.Request,target any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if err:=d.Decode(target);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple JSON values")};return nil}

func (h *Handler) audit(ctx context.Context,userID,projectID,entityID uuid.UUID,action string,payload map[string]any,states ...map[string]any){
  var oldState,newState map[string]any;if len(states)>0{oldState=states[0]};if len(states)>1{newState=states[1]}
  if h.actions!=nil{_,_=h.actions.Create(ctx,userID,projectID,humanactions.Request{ActionType:action,Payload:payload,OldState:oldState,NewState:newState})}
  provenancePayload:=map[string]any{"payload":payload};if oldState!=nil{provenancePayload["old_state"]=oldState};if newState!=nil{provenancePayload["new_state"]=newState}
  if h.provenance!=nil{_,_=h.provenance.Append(ctx,provenance.Event{UserID:userID,ProjectID:projectID,EntityType:"variant",EntityID:entityID,Action:strings.ToLower(action),Payload:provenancePayload})}
  if h.events!=nil{_,_=h.events.Append(ctx,userID,projectID,strings.ToLower(action),"variant",entityID,payload)}
}

func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message,"request_id":uuid.NewString()}})}
