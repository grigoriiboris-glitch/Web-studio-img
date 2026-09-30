package assetlibrary

import (
  "bytes"
  "context"
  "database/sql"
  "encoding/json"
  "errors"
  "fmt"
  "io"
  "net/http"
  "strings"
  "time"

  "github.com/google/uuid"
  "github.com/oleg3190/Web-studio-img/backend/internal/auth"
  "github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

func (h *Handler) Register(mux *http.ServeMux) {
  mux.HandleFunc("GET /api/v1/library/assets", h.list)
  mux.HandleFunc("POST /api/v1/library/assets", h.create)
  mux.HandleFunc("GET /api/v1/library/assets/{item_id}", h.get)
  mux.HandleFunc("PATCH /api/v1/library/assets/{item_id}", h.update)
  mux.HandleFunc("DELETE /api/v1/library/assets/{item_id}", h.archive)
  mux.HandleFunc("GET /api/v1/library/assets/{item_id}/versions", h.listVersionsHTTP)
  mux.HandleFunc("POST /api/v1/library/assets/{item_id}/versions", h.createVersion)
  mux.HandleFunc("GET /api/v1/library/assets/{item_id}/usage", h.listUsage)
  mux.HandleFunc("POST /api/v1/library/assets/{item_id}/use", h.useInProject)
  mux.HandleFunc("GET /api/v1/library/sources", h.listSources)
}

func (h *Handler) userID(r *http.Request) (uuid.UUID, bool) {
  principal, ok := auth.PrincipalFromContext(r.Context())
  if !ok {
    return uuid.Nil, false
  }
  return principal.UserID, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(status)
  _ = json.NewEncoder(w).Encode(value)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
  writeJSON(w, status, map[string]any{
    "error": map[string]string{"code": code, "message": message, "request_id": uuid.NewString()},
  })
}

func readJSON(r *http.Request, dst any) error {
  d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
  d.DisallowUnknownFields()
  if err := d.Decode(dst); err != nil {
    return err
  }
  var extra any
  if err := d.Decode(&extra); err != io.EOF {
    return errors.New("multiple JSON values")
  }
  return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
  uid, ok := h.userID(r)
  if !ok {
    writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
    return
  }
  q := strings.TrimSpace(r.URL.Query().Get("q"))
  assetType := strings.TrimSpace(r.URL.Query().Get("type"))
  tag := strings.TrimSpace(r.URL.Query().Get("tag"))
  status := strings.TrimSpace(r.URL.Query().Get("status"))
  if status == "" {
    status = "active"
  }
  if status != "active" && status != "archived" {
    writeErr(w, http.StatusBadRequest, "invalid_status", "status must be active or archived")
    return
  }
  if assetType != "" && !validType(assetType) {
    writeErr(w, http.StatusBadRequest, "invalid_asset_type", "unsupported asset type")
    return
  }

  query := "SELECT i.id,i.user_id,i.name,i.description,i.asset_type,i.tags,i.status,i.current_version,i.created_at,i.updated_at," +
    "v.id,v.library_item_id,v.version,v.source_asset_id,v.source_project_id,v.storage_key,v.preview_key,v.thumbnail_key," +
    "v.mime_type,v.size,v.width,v.height,v.checksum,v.rights_snapshot,v.provenance,v.created_at " +
    "FROM asset_library_items i JOIN asset_library_versions v ON v.library_item_id=i.id AND v.version=i.current_version " +
    "WHERE i.user_id=$1 AND i.status=$2"
  args := []any{uid, status}
  arg := 3
  if q != "" {
    query += fmt.Sprintf(" AND (i.name ILIKE $%d OR COALESCE(i.description,'') ILIKE $%d)", arg, arg)
    args = append(args, "%"+q+"%")
    arg++
  }
  if assetType != "" {
    query += fmt.Sprintf(" AND i.asset_type=$%d", arg)
    args = append(args, assetType)
    arg++
  }
  if tag != "" {
    query += fmt.Sprintf(" AND i.tags ? $%d", arg)
    args = append(args, strings.ToLower(tag))
    arg++
  }
  query += " ORDER BY i.updated_at DESC LIMIT 200"

  rows, err := h.db.QueryContext(r.Context(), query, args...)
  if err != nil {
    writeErr(w, http.StatusInternalServerError, "library_list_failed", "could not list library assets")
    return
  }
  defer rows.Close()

  items := make([]Item, 0)
  for rows.Next() {
    item, version, err := scanItemVersion(rows)
    if err != nil {
      writeErr(w, http.StatusInternalServerError, "library_list_failed", "could not read library assets")
      return
    }
    _ = h.decorateVersion(r.Context(), &version)
    item.Current = &version
    items = append(items, item)
  }
  if err := rows.Err(); err != nil {
    writeErr(w, http.StatusInternalServerError, "library_list_failed", "could not read library assets")
    return
  }
  writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func scanItemVersion(scanner interface{ Scan(...any) error }) (Item, Version, error) {
  var item Item
  var version Version
  var tagsRaw, rightsRaw, provenanceRaw []byte
  var description sql.NullString
  var sourceAsset, sourceProject uuid.NullUUID
  var previewRaw, thumbRaw sql.NullString

  if err := scanner.Scan(
    &item.ID, &item.UserID, &item.Name, &description, &item.AssetType, &tagsRaw, &item.Status, &item.CurrentVersion, &item.CreatedAt, &item.UpdatedAt,
    &version.ID, &version.LibraryItemID, &version.Version, &sourceAsset, &sourceProject, &version.StorageKey, &previewRaw, &thumbRaw,
    &version.MIMEType, &version.Size, &version.Width, &version.Height, &version.Checksum, &rightsRaw, &provenanceRaw, &version.CreatedAt,
  ); err != nil {
    return Item{}, Version{}, err
  }
  if description.Valid { item.Description = &description.String }
  if sourceAsset.Valid { value := sourceAsset.UUID; version.SourceAssetID = &value }
  if sourceProject.Valid { value := sourceProject.UUID; version.SourceProjectID = &value }
  if previewRaw.Valid { value := previewRaw.String; version.PreviewKey = &value }
  if thumbRaw.Valid { value := thumbRaw.String; version.ThumbnailKey = &value }
  if err := json.Unmarshal(tagsRaw, &item.Tags); err != nil { item.Tags = []string{} }
  if err := json.Unmarshal(rightsRaw, &version.RightsSnapshot); err != nil { version.RightsSnapshot = map[string]any{} }
  if err := json.Unmarshal(provenanceRaw, &version.Provenance); err != nil { version.Provenance = map[string]any{} }
  return item, version, nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
  uid, ok := h.userID(r)
  if !ok { writeErr(w, 401, "unauthorized", "authentication required"); return }
  itemID, err := uuid.Parse(r.PathValue("item_id"))
  if err != nil { writeErr(w, 400, "invalid_item_id", "invalid library item id"); return }

  var item Item
  var version Version
  var tagsRaw, rightsRaw, provenanceRaw []byte
  var description sql.NullString
  var sourceAsset, sourceProject uuid.NullUUID
  var previewRaw, thumbRaw sql.NullString
  query := "SELECT i.id,i.user_id,i.name,i.description,i.asset_type,i.tags,i.status,i.current_version,i.created_at,i.updated_at," +
    "v.id,v.library_item_id,v.version,v.source_asset_id,v.source_project_id,v.storage_key,v.preview_key,v.thumbnail_key," +
    "v.mime_type,v.size,v.width,v.height,v.checksum,v.rights_snapshot,v.provenance,v.created_at " +
    "FROM asset_library_items i JOIN asset_library_versions v ON v.library_item_id=i.id AND v.version=i.current_version " +
    "WHERE i.id=$1 AND i.user_id=$2"
  err = h.db.QueryRowContext(r.Context(), query, itemID, uid).Scan(
    &item.ID, &item.UserID, &item.Name, &description, &item.AssetType, &tagsRaw, &item.Status, &item.CurrentVersion, &item.CreatedAt, &item.UpdatedAt,
    &version.ID, &version.LibraryItemID, &version.Version, &sourceAsset, &sourceProject, &version.StorageKey, &previewRaw, &thumbRaw,
    &version.MIMEType, &version.Size, &version.Width, &version.Height, &version.Checksum, &rightsRaw, &provenanceRaw, &version.CreatedAt,
  )
  if errors.Is(err, sql.ErrNoRows) { writeErr(w, 404, "library_item_not_found", "library item not found"); return }
  if err != nil { writeErr(w, 500, "library_get_failed", "could not load library asset"); return }
  if description.Valid { item.Description = &description.String }
  if sourceAsset.Valid { value:=sourceAsset.UUID; version.SourceAssetID=&value }
  if sourceProject.Valid { value:=sourceProject.UUID; version.SourceProjectID=&value }
  if previewRaw.Valid { value:=previewRaw.String; version.PreviewKey=&value }
  if thumbRaw.Valid { value:=thumbRaw.String; version.ThumbnailKey=&value }
  _=json.Unmarshal(tagsRaw,&item.Tags)
  _=json.Unmarshal(rightsRaw,&version.RightsSnapshot)
  _=json.Unmarshal(provenanceRaw,&version.Provenance)
  _=h.decorateVersion(r.Context(),&version)
  item.Current=&version
  versions,err:=h.listVersions(r.Context(),uid,itemID)
  if err!=nil { writeErr(w,500,"library_versions_failed","could not load library versions"); return }
  usage,err:=h.loadUsage(r.Context(),uid,itemID)
  if err!=nil { writeErr(w,500,"library_usage_failed","could not load project usage"); return }
  writeJSON(w,200,map[string]any{"item":item,"versions":versions,"usage":usage})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  itemID,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid library item id");return}
  var in struct{Name string `json:"name"`;Description *string `json:"description"`;AssetType string `json:"asset_type"`;Tags []string `json:"tags"`;Status string `json:"status"`}
  if err:=readJSON(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid library asset payload");return}
  in.Name=strings.TrimSpace(in.Name);in.AssetType=strings.TrimSpace(in.AssetType);in.Status=strings.TrimSpace(in.Status)
  if in.Name==""||len([]rune(in.Name))>200||!validType(in.AssetType)||(in.Status!="active"&&in.Status!="archived"){writeErr(w,400,"invalid_library_item","invalid library asset");return}
  tags:=normalizeTags(in.Tags);raw,_:=json.Marshal(tags)
  var item Item;var description sql.NullString
  err=h.db.QueryRowContext(r.Context(),
    "UPDATE asset_library_items SET name=$1,description=$2,asset_type=$3,tags=$4,status=$5,updated_at=now() WHERE id=$6 AND user_id=$7 "+
      "RETURNING id,user_id,name,description,asset_type,tags,status,current_version,created_at,updated_at",
    in.Name,in.Description,in.AssetType,raw,in.Status,itemID,uid,
  ).Scan(&item.ID,&item.UserID,&item.Name,&description,&item.AssetType,&raw,&item.Status,&item.CurrentVersion,&item.CreatedAt,&item.UpdatedAt)
  if errors.Is(err,sql.ErrNoRows){writeErr(w,404,"library_item_not_found","library item not found");return}
  if err!=nil{writeErr(w,500,"library_update_failed","could not update library asset");return}
  if description.Valid{item.Description=&description.String};_ = json.Unmarshal(raw,&item.Tags)
  h.event(r.Context(), &uid, &item.ID,nil,nil,"asset_library.updated",map[string]any{"status":item.Status})
  writeJSON(w,200,item)
}

func (h *Handler) archive(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  itemID,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid library item id");return}
  result,err:=h.db.ExecContext(r.Context(),"UPDATE asset_library_items SET status='archived',updated_at=now() WHERE id=$1 AND user_id=$2",itemID,uid)
  if err!=nil{writeErr(w,500,"library_archive_failed","could not archive library asset");return}
  count,_:=result.RowsAffected();if count!=1{writeErr(w,404,"library_item_not_found","library item not found");return}
  h.event(r.Context(), &uid, &itemID,nil,nil,"asset_library.archived",nil)
  w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) create(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  if h.storage==nil{writeErr(w,503,"storage_unavailable","object storage is not configured");return}
  var in struct{SourceAssetID uuid.UUID `json:"source_asset_id"`;Name string `json:"name"`;Description *string `json:"description"`;AssetType string `json:"asset_type"`;Tags []string `json:"tags"`}
  if err:=readJSON(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid library asset payload");return}
  in.Name=strings.TrimSpace(in.Name);in.AssetType=strings.TrimSpace(in.AssetType)
  if in.SourceAssetID==uuid.Nil||in.Name==""||!validType(in.AssetType){writeErr(w,400,"invalid_library_item","source_asset_id, name and valid asset_type are required");return}
  if len([]rune(in.Name))>200{writeErr(w,400,"invalid_library_item","name is too long");return}
  source,rights,err:=h.loadSource(r.Context(),uid,in.SourceAssetID);if err!=nil{if errors.Is(err,errSourceAssetNotFound){writeErr(w,404,"source_asset_not_found","source asset not found");return};writeErr(w,500,"source_asset_load_failed","could not load source asset");return}
  tags:=normalizeTags(in.Tags);tagsRaw,_:=json.Marshal(tags)
  itemID:=uuid.New();versionID:=uuid.New()
  versionKey:=fmt.Sprintf("users/%s/asset-library/%s/v1/original",uid,itemID)
  previewKey:=fmt.Sprintf("users/%s/asset-library/%s/v1/preview",uid,itemID)
  thumbKey:=fmt.Sprintf("users/%s/asset-library/%s/v1/thumbnail",uid,itemID)
  reused:=false
  var existing struct{StorageKey string;PreviewKey,ThumbnailKey sql.NullString}
  dedupeErr:=h.db.QueryRowContext(r.Context(),
    "SELECT v.storage_key,v.preview_key,v.thumbnail_key FROM asset_library_versions v JOIN asset_library_items i ON i.id=v.library_item_id WHERE i.user_id=$1 AND v.checksum=$2 ORDER BY v.created_at LIMIT 1",
    uid,source.Checksum,
  ).Scan(&existing.StorageKey,&existing.PreviewKey,&existing.ThumbnailKey)
  if dedupeErr != nil && !errors.Is(dedupeErr, sql.ErrNoRows) { writeErr(w, 500, "library_dedupe_lookup_failed", "could not check library deduplication"); return }
  if dedupeErr==nil{reused=true;versionKey=existing.StorageKey;if existing.PreviewKey.Valid{previewKey=existing.PreviewKey.String}else{previewKey=""};if existing.ThumbnailKey.Valid{thumbKey=existing.ThumbnailKey.String}else{thumbKey=""}}
  if !reused{
    if err:=copyObject(r.Context(),h.storage,source.StorageKey,versionKey,source.MIMEType,source.Checksum);err!=nil{writeErr(w,502,"library_copy_failed","could not copy source asset");return}
    if source.PreviewKey!=nil{if err:=copyObject(r.Context(),h.storage,*source.PreviewKey,previewKey,"image/jpeg",source.Checksum);err!=nil{cleanupKeys(h.storage,versionKey);writeErr(w,502,"library_copy_failed","could not copy preview");return}}else{previewKey=""}
    if source.ThumbnailKey!=nil{if err:=copyObject(r.Context(),h.storage,*source.ThumbnailKey,thumbKey,"image/jpeg",source.Checksum);err!=nil{cleanupKeys(h.storage,versionKey,previewKey);writeErr(w,502,"library_copy_failed","could not copy thumbnail");return}}else{thumbKey=""}
  }
  rightsRaw,_:=json.Marshal(rights);provenance:=map[string]any{"source_asset_id":source.ID,"source_project_id":source.ProjectID,"rights_snapshot":rights,"deduplicated":reused};provenanceRaw,_:=json.Marshal(provenance)
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{if !reused{cleanupKeys(h.storage,versionKey,previewKey,thumbKey)};writeErr(w,500,"library_create_failed","could not start library transaction");return}
  _,err=tx.ExecContext(r.Context(),"INSERT INTO asset_library_items(id,user_id,name,description,asset_type,tags,status,current_version) VALUES($1,$2,$3,$4,$5,$6,'active',1)",itemID,uid,in.Name,in.Description,in.AssetType,tagsRaw)
  if err==nil{_,err=tx.ExecContext(r.Context(),"INSERT INTO asset_library_versions(id,library_item_id,version,source_asset_id,source_project_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,rights_snapshot,provenance) VALUES($1,$2,1,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14)",versionID,itemID,source.ID,source.ProjectID,versionKey,previewKey,thumbKey,source.MIMEType,source.Size,source.Width,source.Height,source.Checksum,rightsRaw,provenanceRaw)}
  if err!=nil{_ = tx.Rollback();if !reused{cleanupKeys(h.storage,versionKey,previewKey,thumbKey)};writeErr(w,500,"library_create_failed","could not create library asset");return}
  if err:=tx.Commit();err!=nil{if !reused{cleanupKeys(h.storage,versionKey,previewKey,thumbKey)};writeErr(w,500,"library_create_failed","could not commit library asset");return}
  h.event(r.Context(), &uid, &itemID,&versionID,nil,"asset_library.created",map[string]any{"source_asset_id":source.ID})
  version:=Version{ID:versionID,LibraryItemID:itemID,Version:1,SourceAssetID:&source.ID,SourceProjectID:&source.ProjectID,StorageKey:versionKey,MIMEType:source.MIMEType,Size:source.Size,Width:source.Width,Height:source.Height,Checksum:source.Checksum,RightsSnapshot:rights,Provenance:provenance}
  if previewKey!=""{version.PreviewKey=&previewKey};if thumbKey!=""{version.ThumbnailKey=&thumbKey};_=h.decorateVersion(r.Context(),&version)
  item:=Item{ID:itemID,UserID:uid,Name:in.Name,Description:in.Description,AssetType:in.AssetType,Tags:tags,Status:"active",CurrentVersion:1,Current:&version}
  writeJSON(w,201,item)
}

func (h *Handler) loadSource(ctx context.Context,uid,assetID uuid.UUID)(SourceAssetDB,map[string]any,error){
  var source SourceAssetDB
  var preview,thumb,checksum sql.NullString
  err:=h.db.QueryRowContext(ctx,
    "SELECT a.id,a.project_id,a.storage_key,a.preview_key,a.thumbnail_key,a.mime_type,a.size,a.width,a.height,a.checksum,a.created_at FROM assets a JOIN projects p ON p.id=a.project_id WHERE a.id=$1 AND a.user_id=$2 AND p.user_id=$2 AND p.status <> 'deleted' AND a.lifecycle_status='active'",
    assetID,uid,
  ).Scan(&source.ID,&source.ProjectID,&source.StorageKey,&preview,&thumb,&source.MIMEType,&source.Size,&source.Width,&source.Height,&checksum,&source.CreatedAt)
  if errors.Is(err,sql.ErrNoRows){return SourceAssetDB{},nil,errSourceAssetNotFound}
  if err!=nil{return SourceAssetDB{},nil,err}
  if preview.Valid{value:=preview.String;source.PreviewKey=&value};if thumb.Valid{value:=thumb.String;source.ThumbnailKey=&value}
  if !checksum.Valid||strings.TrimSpace(checksum.String)==""{return SourceAssetDB{},nil,errors.New("source asset has no checksum")}
  source.Checksum=checksum.String
  rights:=map[string]any{"ownership":"unknown","license":"unknown","verification_state":"unknown"}
  var ownership,license,licenseSource,verification,notes string
  var verificationDate *time.Time
  rightsErr:=h.db.QueryRowContext(ctx,"SELECT ownership,license,COALESCE(license_source,''),verification_state,COALESCE(notes,''),verification_date FROM rights_registry WHERE user_id=$1 AND target_type='asset' AND target_id=$2",uid,assetID).Scan(&ownership,&license,&licenseSource,&verification,&notes,&verificationDate)
  if rightsErr==nil{rights=map[string]any{"ownership":ownership,"license":license,"license_source":licenseSource,"verification_state":verification,"notes":notes,"verification_date":verificationDate}}
  return source,rights,nil
}

func (h *Handler) listVersions(ctx context.Context,uid,itemID uuid.UUID)([]Version,error){
  rows,err:=h.db.QueryContext(ctx,
    "SELECT v.id,v.library_item_id,v.version,v.source_asset_id,v.source_project_id,v.storage_key,v.preview_key,v.thumbnail_key,v.mime_type,v.size,v.width,v.height,v.checksum,v.rights_snapshot,v.provenance,v.created_at FROM asset_library_versions v JOIN asset_library_items i ON i.id=v.library_item_id WHERE i.id=$1 AND i.user_id=$2 ORDER BY v.version DESC",
    itemID,uid,
  )
  if err!=nil{return nil,err};defer rows.Close()
  out:=make([]Version,0)
  for rows.Next(){
    var v Version;var sourceAsset,sourceProject uuid.NullUUID;var preview,thumb sql.NullString;var rightsRaw,provRaw []byte
    if err:=rows.Scan(&v.ID,&v.LibraryItemID,&v.Version,&sourceAsset,&sourceProject,&v.StorageKey,&preview,&thumb,&v.MIMEType,&v.Size,&v.Width,&v.Height,&v.Checksum,&rightsRaw,&provRaw,&v.CreatedAt);err!=nil{return nil,err}
    if sourceAsset.Valid{value:=sourceAsset.UUID;v.SourceAssetID=&value};if sourceProject.Valid{value:=sourceProject.UUID;v.SourceProjectID=&value}
    if preview.Valid{value:=preview.String;v.PreviewKey=&value};if thumb.Valid{value:=thumb.String;v.ThumbnailKey=&value}
    _=json.Unmarshal(rightsRaw,&v.RightsSnapshot);_=json.Unmarshal(provRaw,&v.Provenance);_=h.decorateVersion(ctx,&v);out=append(out,v)
  }
  return out,rows.Err()
}

func (h *Handler) listVersionsHTTP(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  itemID,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid library item id");return}
  versions,err:=h.listVersions(r.Context(),uid,itemID);if err!=nil{if errors.Is(err,sql.ErrNoRows){writeErr(w,404,"library_item_not_found","library item not found");return};writeErr(w,500,"library_versions_failed","could not list library versions");return}
  if versions==nil{versions=[]Version{}}
  writeJSON(w,200,map[string]any{"versions":versions})
}

func (h *Handler) createVersion(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  if h.storage==nil{writeErr(w,503,"storage_unavailable","object storage is not configured");return}
  itemID,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid library item id");return}
  var in struct{SourceAssetID uuid.UUID `json:"source_asset_id"`}
  if err:=readJSON(r,&in);err!=nil||in.SourceAssetID==uuid.Nil{writeErr(w,400,"invalid_request","source_asset_id is required");return}
  source,rights,err:=h.loadSource(r.Context(),uid,in.SourceAssetID);if err!=nil{writeErr(w,404,"source_asset_not_found","source asset not found");return}
  var itemUser uuid.UUID;var status string;var next int
  err=h.db.QueryRowContext(r.Context(),"SELECT user_id,status,current_version+1 FROM asset_library_items WHERE id=$1",itemID).Scan(&itemUser,&status,&next)
  if errors.Is(err,sql.ErrNoRows)||itemUser!=uid{writeErr(w,404,"library_item_not_found","library item not found");return}
  if status=="archived"{writeErr(w,409,"library_item_archived","archived library item cannot receive a new version");return}
  versionID:=uuid.New();versionKey:=fmt.Sprintf("users/%s/asset-library/%s/v%d/original",uid,itemID,next);previewKey:=fmt.Sprintf("users/%s/asset-library/%s/v%d/preview",uid,itemID,next);thumbKey:=fmt.Sprintf("users/%s/asset-library/%s/v%d/thumbnail",uid,itemID,next)
  reused:=false
  var existing struct{StorageKey string;PreviewKey,ThumbnailKey sql.NullString}
  dedupeErr:=h.db.QueryRowContext(r.Context(),"SELECT v.storage_key,v.preview_key,v.thumbnail_key FROM asset_library_versions v JOIN asset_library_items i ON i.id=v.library_item_id WHERE i.user_id=$1 AND v.checksum=$2 ORDER BY v.created_at LIMIT 1",uid,source.Checksum).Scan(&existing.StorageKey,&existing.PreviewKey,&existing.ThumbnailKey)
  if dedupeErr != nil && !errors.Is(dedupeErr, sql.ErrNoRows) { writeErr(w, 500, "library_dedupe_lookup_failed", "could not check library deduplication"); return }
  if dedupeErr==nil{reused=true;versionKey=existing.StorageKey;if existing.PreviewKey.Valid{previewKey=existing.PreviewKey.String}else{previewKey=""};if existing.ThumbnailKey.Valid{thumbKey=existing.ThumbnailKey.String}else{thumbKey=""}}
  if !reused{
    if err:=copyObject(r.Context(),h.storage,source.StorageKey,versionKey,source.MIMEType,source.Checksum);err!=nil{writeErr(w,502,"library_copy_failed","could not copy source asset");return}
    if source.PreviewKey!=nil{if err:=copyObject(r.Context(),h.storage,*source.PreviewKey,previewKey,"image/jpeg",source.Checksum);err!=nil{cleanupKeys(h.storage,versionKey);writeErr(w,502,"library_copy_failed","could not copy preview");return}}else{previewKey=""}
    if source.ThumbnailKey!=nil{if err:=copyObject(r.Context(),h.storage,*source.ThumbnailKey,thumbKey,"image/jpeg",source.Checksum);err!=nil{cleanupKeys(h.storage,versionKey,previewKey);writeErr(w,502,"library_copy_failed","could not copy thumbnail");return}}else{thumbKey=""}
  }
  rightsRaw,_:=json.Marshal(rights);provenance:=map[string]any{"source_asset_id":source.ID,"source_project_id":source.ProjectID,"rights_snapshot":rights,"deduplicated":reused};provenanceRaw,_:=json.Marshal(provenance)
  tx,err:=h.db.BeginTx(r.Context(),nil);if err!=nil{if !reused{cleanupKeys(h.storage,versionKey,previewKey,thumbKey)};writeErr(w,500,"library_version_create_failed","could not start version transaction");return}
  _,err=tx.ExecContext(r.Context(),"INSERT INTO asset_library_versions(id,library_item_id,version,source_asset_id,source_project_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,rights_snapshot,provenance) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),$9,$10,$11,$12,$13,$14,$15)",versionID,itemID,next,source.ID,source.ProjectID,versionKey,previewKey,thumbKey,source.MIMEType,source.Size,source.Width,source.Height,source.Checksum,rightsRaw,provenanceRaw)
  if err==nil{_,err=tx.ExecContext(r.Context(),"UPDATE asset_library_items SET current_version=$1,updated_at=now() WHERE id=$2 AND user_id=$3",next,itemID,uid)}
  if err!=nil{_ = tx.Rollback();if !reused{cleanupKeys(h.storage,versionKey,previewKey,thumbKey)};writeErr(w,500,"library_version_create_failed","could not create library version");return}
  if err=tx.Commit();err!=nil{if !reused{cleanupKeys(h.storage,versionKey,previewKey,thumbKey)};writeErr(w,500,"library_version_create_failed","could not commit library version");return}
  h.event(r.Context(), &uid, &itemID,&versionID,&source.ProjectID,"asset_library.version_created",map[string]any{"source_asset_id":source.ID,"version":next})
  v:=Version{ID:versionID,LibraryItemID:itemID,Version:next,SourceAssetID:&source.ID,SourceProjectID:&source.ProjectID,StorageKey:versionKey,MIMEType:source.MIMEType,Size:source.Size,Width:source.Width,Height:source.Height,Checksum:source.Checksum,RightsSnapshot:rights,Provenance:provenance}
  if previewKey!=""{v.PreviewKey=&previewKey};if thumbKey!=""{v.ThumbnailKey=&thumbKey};_=h.decorateVersion(r.Context(),&v);writeJSON(w,201,v)
}

func (h *Handler) useInProject(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  if h.storage==nil{writeErr(w,503,"storage_unavailable","object storage is not configured");return}
  itemID,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid library item id");return}
  var in struct{ProjectID uuid.UUID `json:"project_id"`;Version int `json:"version"`;RightsStatus string `json:"rights_status"`;RightsNotes *string `json:"rights_notes"`}
  if err:=readJSON(r,&in);err!=nil{writeErr(w,400,"invalid_request","invalid library usage payload");return}
  if in.ProjectID==uuid.Nil{writeErr(w,400,"invalid_project_id","project_id is required");return}
  if in.RightsStatus==""{in.RightsStatus="inherited"};if !validRights(in.RightsStatus){writeErr(w,400,"invalid_rights_status","invalid rights status");return}
  var versionID,itemUser uuid.UUID;var version int;var storageKey,mimeType,checksum string;var previewKey,thumbKey sql.NullString;var size int64;var width,height int
  err=h.db.QueryRowContext(r.Context(),"SELECT v.id,i.user_id,v.version,v.storage_key,v.preview_key,v.thumbnail_key,v.mime_type,v.size,v.width,v.height,v.checksum FROM asset_library_versions v JOIN asset_library_items i ON i.id=v.library_item_id WHERE i.id=$1 AND i.user_id=$2 AND i.status='active' AND ($3=0 OR v.version=$3)",itemID,uid,in.Version).Scan(&versionID,&itemUser,&version,&storageKey,&previewKey,&thumbKey,&mimeType,&size,&width,&height,&checksum)
  if errors.Is(err,sql.ErrNoRows){writeErr(w,404,"library_version_not_found","library asset or version not found");return};if err!=nil{writeErr(w,500,"library_use_failed","could not load library version");return};if itemUser!=uid{writeErr(w,404,"library_version_not_found","library asset not found");return}
  if err:=h.db.QueryRowContext(r.Context(),"SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted'",in.ProjectID,uid).Scan(new(int));err!=nil{writeErr(w,404,"project_not_found","target project not found");return}
  var usage Usage
  err=h.db.QueryRowContext(r.Context(),"INSERT INTO asset_library_usages(library_version_id,user_id,project_id,rights_status,rights_notes) VALUES($1,$2,$3,$4,$5) ON CONFLICT(library_version_id,project_id) DO UPDATE SET rights_status=excluded.rights_status,rights_notes=excluded.rights_notes RETURNING id,library_version_id,user_id,project_id,rights_status,rights_notes,created_at",versionID,uid,in.ProjectID,in.RightsStatus,in.RightsNotes).Scan(&usage.ID,&usage.LibraryVersionID,&usage.UserID,&usage.ProjectID,&usage.RightsStatus,&usage.RightsNotes,&usage.CreatedAt)
  if err!=nil{writeErr(w,500,"library_use_failed","could not record project usage");return}
  h.event(r.Context(), &uid, &itemID,&versionID,&in.ProjectID,"asset_library.used_in_project",map[string]any{"version":version,"rights_status":in.RightsStatus})
  v:=Version{ID:versionID,LibraryItemID:itemID,Version:version,StorageKey:storageKey,MIMEType:mimeType,Size:size,Width:width,Height:height,Checksum:checksum};if previewKey.Valid{x:=previewKey.String;v.PreviewKey=&x};if thumbKey.Valid{x:=thumbKey.String;v.ThumbnailKey=&x};_=h.decorateVersion(r.Context(),&v)
  writeJSON(w,200,map[string]any{"usage":usage,"version":v})
}

func (h *Handler) listUsage(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  itemID,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid library item id");return}
  usage,err:=h.loadUsage(r.Context(),uid,itemID);if err!=nil{writeErr(w,500,"library_usage_failed","could not list project usage");return}
  writeJSON(w,200,map[string]any{"usage":usage})
}

func (h *Handler) loadUsage(ctx context.Context,uid,itemID uuid.UUID)([]Usage,error){
  rows,err:=h.db.QueryContext(ctx,"SELECT u.id,u.library_version_id,u.user_id,u.project_id,p.name,u.rights_status,u.rights_notes,u.created_at FROM asset_library_usages u JOIN asset_library_versions v ON v.id=u.library_version_id JOIN asset_library_items i ON i.id=v.library_item_id JOIN projects p ON p.id=u.project_id WHERE i.id=$1 AND i.user_id=$2 ORDER BY u.created_at DESC",itemID,uid)
  if err!=nil{return nil,err};defer rows.Close()
  out:=make([]Usage,0);for rows.Next(){var item Usage;if err:=rows.Scan(&item.ID,&item.LibraryVersionID,&item.UserID,&item.ProjectID,&item.ProjectName,&item.RightsStatus,&item.RightsNotes,&item.CreatedAt);err!=nil{return nil,err};out=append(out,item)}
  return out,rows.Err()
}

func (h *Handler) listSources(w http.ResponseWriter,r *http.Request){
  uid,ok:=h.userID(r);if !ok{writeErr(w,401,"unauthorized","authentication required");return}
  projectID,err:=uuid.Parse(strings.TrimSpace(r.URL.Query().Get("project_id")));if err!=nil{writeErr(w,400,"invalid_project_id","project_id is required");return}
  rows,err:=h.db.QueryContext(r.Context(),"SELECT a.id,a.project_id,a.type,a.mime_type,a.size,a.width,a.height,a.checksum,a.preview_key,a.created_at FROM assets a JOIN projects p ON p.id=a.project_id WHERE a.project_id=$1 AND a.user_id=$2 AND p.status <> 'deleted' AND a.lifecycle_status='active' ORDER BY a.created_at DESC LIMIT 200",projectID,uid)
  if err!=nil{writeErr(w,500,"library_sources_failed","could not list source assets");return};defer rows.Close()
  out:=make([]SourceAsset,0)
  for rows.Next(){var item SourceAsset;var preview sql.NullString;if err:=rows.Scan(&item.ID,&item.ProjectID,&item.Type,&item.MIMEType,&item.Size,&item.Width,&item.Height,&item.Checksum,&preview,&item.CreatedAt);err!=nil{writeErr(w,500,"library_sources_failed","could not read source assets");return};if preview.Valid&&h.storage!=nil{if signed,e:=h.storage.PresignGet(r.Context(),preview.String,10*time.Minute);e==nil{item.PreviewURL=signed.URL}};out=append(out,item)}
  if err:=rows.Err();err!=nil{writeErr(w,500,"library_sources_failed","could not read source assets");return}
  writeJSON(w,200,map[string]any{"sources":out})
}

func (h *Handler) decorateVersion(ctx context.Context,v *Version) error{
  if h.storage==nil{return nil}
  if v.StorageKey!=""{if signed,err:=h.storage.PresignGet(ctx,v.StorageKey,10*time.Minute);err==nil{v.OriginalURL=signed.URL}}
  if v.PreviewKey!=nil&&*v.PreviewKey!=""{if signed,err:=h.storage.PresignGet(ctx,*v.PreviewKey,10*time.Minute);err==nil{v.PreviewURL=signed.URL}}
  if v.ThumbnailKey!=nil&&*v.ThumbnailKey!=""{if signed,err:=h.storage.PresignGet(ctx,*v.ThumbnailKey,10*time.Minute);err==nil{v.ThumbnailURL=signed.URL}}
  return nil
}

func (h *Handler) event(ctx context.Context,uid,itemID,versionID,projectID *uuid.UUID,eventType string,payload map[string]any){
  raw,_:=json.Marshal(payload)
  _,_=h.db.ExecContext(ctx,"INSERT INTO asset_library_events(user_id,library_item_id,library_version_id,project_id,event_type,payload) VALUES($1,$2,$3,$4,$5,$6)",uid,itemID,versionID,projectID,eventType,raw)
}

func copyObject(ctx context.Context,p storage.StorageProvider,src,dst,contentType,checksum string) error{
  object,info,err:=p.Get(ctx,src);if err!=nil{return err};defer object.Close()
  data,err:=io.ReadAll(object);if err!=nil{return err};if contentType==""{contentType=info.ContentType}
  return p.Put(ctx,dst,bytes.NewReader(data),int64(len(data)),storage.PutOptions{ContentType:contentType,Metadata:map[string]string{"sha256":checksum}})
}

func cleanupKeys(p storage.StorageProvider,keys ...string){if p==nil{return};for _,key:=range keys{if strings.TrimSpace(key)!=""{_=p.Delete(context.Background(),key)}}}
