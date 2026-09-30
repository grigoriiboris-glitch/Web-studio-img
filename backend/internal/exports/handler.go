package exports

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Handler struct {
	db         *sql.DB
	assets     *assets.Store
	storage    storage.StorageProvider
	events     *events.Store
	provenance *provenance.Store
	actions    *humanactions.Store
}

type Export struct {
	ID              uuid.UUID          `json:"id"`
	ProjectID       uuid.UUID          `json:"project_id"`
	UserID          uuid.UUID          `json:"user_id"`
	FinalAssetID    uuid.UUID          `json:"final_asset_id"`
	FinalIterationID *uuid.UUID        `json:"final_iteration_id,omitempty"`
	Status          string             `json:"status"`
	Artifacts   map[string]string  `json:"artifacts"`
	Manifest     map[string]any    `json:"manifest"`
	Error        string             `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	IdempotencyKey string `json:"-"`
}

func NewHandler(a *assets.Store, s storage.StorageProvider, e *events.Store, p *provenance.Store, h *humanactions.Store) (*Handler, error) {
	return NewHandlerWithDB(nil, a, s, e, p, h)
}
func NewHandlerWithDB(db *sql.DB, a *assets.Store, s storage.StorageProvider, e *events.Store, p *provenance.Store, h *humanactions.Store) (*Handler, error) {
	if a == nil { return nil, errors.New("export handler requires asset store") }
	return &Handler{db:db,assets:a,storage:s,events:e,provenance:p,actions:h}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{project_id}/exports", h.create)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/exports/{export_id}", h.get)
	mux.HandleFunc("GET /api/v1/exports/{export_id}", h.get)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assets/{asset_id}/export", h.legacy)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { errJSON(w,401,"unauthorized","authentication required"); return }
	if h.db == nil || h.storage == nil { errJSON(w,503,"export_unavailable","export storage is not configured"); return }
	if strings.TrimSpace(r.Header.Get("Idempotency-Key")) == "" { errJSON(w,400,"missing_idempotency_key","Idempotency-Key header is required"); return }
	projectID, err := uuid.Parse(r.PathValue("project_id")); if err != nil { errJSON(w,400,"invalid_project_id","invalid project id"); return }
	var in struct { FinalAssetID uuid.UUID `json:"final_asset_id"` }
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r,&in); err != nil { errJSON(w,400,"invalid_request","invalid export payload"); return }
	}
	var workflowState string
	var finalIterationID *uuid.UUID
	var gateAssetID *uuid.UUID
	if err := h.db.QueryRowContext(r.Context(), `SELECT workflow_state,final_iteration_id FROM projects WHERE id=$1 AND user_id=$2 AND status<>'deleted'`, projectID, p.UserID).Scan(&workflowState, &finalIterationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) { errJSON(w,404,"project_not_found","project not found"); return }
		errJSON(w,500,"export_create_failed","could not load project final state"); return
	}
	if workflowState == "final" {
		if err := h.db.QueryRowContext(r.Context(), `SELECT final_asset_id FROM project_approval_gates WHERE project_id=$1 AND user_id=$2`, projectID, p.UserID).Scan(&gateAssetID); err != nil || gateAssetID == nil || *gateAssetID == uuid.Nil {
			errJSON(w,409,"final_asset_not_recorded","final project has no recorded final asset"); return
		}
		if in.FinalAssetID != uuid.Nil && in.FinalAssetID != *gateAssetID {
			errJSON(w,409,"final_asset_mismatch","export must reference the approved final asset"); return
		}
		in.FinalAssetID = *gateAssetID
	}
	assetID := in.FinalAssetID
	if assetID == uuid.Nil {
		if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM assets WHERE project_id=$1 AND user_id=$2 AND lifecycle_status='active' ORDER BY created_at DESC LIMIT 1`, projectID,p.UserID).Scan(&assetID); err != nil {
			if errors.Is(err,sql.ErrNoRows) { errJSON(w,404,"final_asset_not_found","no active final asset found"); return }
			errJSON(w,500,"final_asset_lookup_failed","could not choose final asset"); return
		}
	}
	asset, err := h.assets.GetOwned(r.Context(),p.UserID,projectID,assetID); if err != nil { errJSON(w,404,"final_asset_not_found","final asset not found"); return }
	var out Export
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var artifactRaw, manifestRaw []byte
	err = h.db.QueryRowContext(r.Context(), `INSERT INTO export_records(project_id,user_id,final_asset_id,final_iteration_id,status,idempotency_key) VALUES($1,$2,$3,$4,'running',$5) ON CONFLICT (user_id,idempotency_key) DO NOTHING RETURNING id,project_id,user_id,final_asset_id,final_iteration_id,status,artifacts,manifest,error,created_at,completed_at`, projectID,p.UserID,asset.ID,finalIterationID,idempotencyKey).Scan(&out.ID,&out.ProjectID,&out.UserID,&out.FinalAssetID,&out.FinalIterationID,&out.Status,&artifactRaw,&manifestRaw,&out.Error,&out.CreatedAt,&out.CompletedAt)
	if errors.Is(err,sql.ErrNoRows) {
		var existingAsset uuid.UUID
		if qerr:=h.db.QueryRowContext(r.Context(), `SELECT id,final_asset_id FROM export_records WHERE user_id=$1 AND idempotency_key=$2`,p.UserID,idempotencyKey).Scan(&out.ID,&existingAsset); qerr!=nil { errJSON(w,500,"export_create_failed","could not load idempotent export"); return }
		if existingAsset != asset.ID { errJSON(w,409,"idempotency_conflict","idempotency key was already used for another final asset"); return }
		loaded,qerr:=h.load(r.Context(),p.UserID,out.ID); if qerr!=nil { errJSON(w,500,"export_load_failed","could not load export"); return }
		writeJSON(w,200,loaded); return
	}
	if err != nil { errJSON(w,500,"export_create_failed","could not create export record"); return }
	if err := h.build(r.Context(), p.UserID, projectID, out.ID, asset, finalIterationID); err != nil {
		_,_ = h.db.ExecContext(r.Context(), `UPDATE export_records SET status='failed', error=$2 WHERE id=$1`,out.ID,err.Error())
		errJSON(w,500,"export_failed","could not build creation report"); return
	}
	out, err = h.load(r.Context(),p.UserID,out.ID); if err != nil { errJSON(w,500,"export_load_failed","could not load export"); return }
	h.writeAction(r,p.UserID,projectID,out.ID,map[string]any{"final_asset_id":asset.ID,"final_iteration_id":finalIterationID,"status":out.Status})
	writeJSON(w,201,out)
}

func (h *Handler) build(ctx context.Context,userID,projectID,exportID uuid.UUID,asset assets.Asset,finalIterationID *uuid.UUID) error {
	imageData, _, err := h.storage.Get(ctx,asset.StorageKey); if err != nil { return fmt.Errorf("read final image: %w",err) }
	defer func() { _ = imageData.Close() }()
	source, err := io.ReadAll(io.LimitReader(imageData, assets.MaxAssetSize+1)); if err != nil { return fmt.Errorf("read final image: %w",err) }
	if int64(len(source)) > assets.MaxAssetSize { return errors.New("final image exceeds export limit") }

	prov := []provenance.Event{}
	var verification provenance.Verification
	if h.provenance != nil {
		prov, err = h.provenance.List(ctx,userID,projectID); if err != nil { return fmt.Errorf("load provenance: %w",err) }
		verification, err = h.provenance.Verify(ctx,userID,projectID); if err != nil { return fmt.Errorf("verify provenance: %w",err) }
	}
	timeline, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.sequence),'[]'::jsonb) FROM project_events e WHERE e.project_id=$1 AND e.user_id=$2`,projectID,userID); if err != nil { return err }
	prompts, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.version),'[]'::jsonb) FROM prompts p WHERE p.project_id=$1`,projectID); if err != nil { return err }
	refs, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.created_at),'[]'::jsonb) FROM "references" r WHERE r.project_id=$1`,projectID); if err != nil { return err }
	referenceUsages, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(u) ORDER BY u.created_at),'[]'::jsonb) FROM reference_usages u WHERE u.project_id=$1 AND u.user_id=$2`,projectID,userID); if err != nil { return err }
	actions, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.version),'[]'::jsonb) FROM human_actions a WHERE a.project_id=$1`,projectID); if err != nil { return err }
	similarity, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY s.created_at),'[]'::jsonb) FROM similarity_checks s WHERE s.project_id=$1`,projectID); if err != nil { return err }
	generations, err := h.loadJSON(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(g) ORDER BY g.created_at),'[]'::jsonb) FROM generations g WHERE g.project_id=$1`,projectID); if err != nil { return err }

	reportMeta := map[string]any{
		"export_id": exportID, "project_id": projectID, "final_asset_id": asset.ID, "final_iteration_id": finalIterationID,
		"generated_at": time.Now().UTC(), "legal_note": "Creation Report describes the process and checks; it is not an automatic legal conclusion.",
		"provenance_verification": verification,
		"timeline_count": countJSON(timeline), "prompt_count": countJSON(prompts),
		"reference_count": countJSON(refs), "reference_usage_count": countJSON(referenceUsages), "human_action_count": countJSON(actions),
		"similarity_check_count": countJSON(similarity), "generation_count": countJSON(generations),
	}
	provenanceJSON, _ := json.MarshalIndent(map[string]any{"events":prov,"verification":verification}, "", "  ")
	dataFiles := map[string][]byte{
		"provenance.json": provenanceJSON,
		"timeline.json": timeline,
		"prompts.json": prompts,
		"references.json": refs,
		"reference_usages.json": referenceUsages,
		"human_actions.json": actions,
		"similarity_checks.json": similarity,
		"generations.json": generations,
	}
	manifest := map[string]any{"version":"1","report":reportMeta,"source_files":map[string]any{
		"final_image": map[string]any{"storage_key":asset.StorageKey,"sha256":asset.Checksum,"mime_type":asset.MIMEType,"size":asset.Size,"width":asset.Width,"height":asset.Height},
	}}
	artifactKeys:=map[string]string{}
	hashes:=map[string]string{}
	for name,data := range dataFiles {
		key:=fmt.Sprintf("projects/%s/exports/%s/%s",projectID,exportID,name)
		if err:=h.storage.Put(ctx,key,strings.NewReader(string(data)),int64(len(data)),storage.PutOptions{ContentType:"application/json"});err!=nil{return fmt.Errorf("write %s: %w",name,err)}
		artifactKeys[name]=key; hashes[name]=sha256Hex(data)
	}
	sourceKey:=fmt.Sprintf("projects/%s/exports/%s/source/final-image",projectID,exportID)
	if err:=h.storage.Put(ctx,sourceKey,strings.NewReader(string(source)),int64(len(source)),storage.PutOptions{ContentType:asset.MIMEType});err!=nil{return fmt.Errorf("write final source: %w",err)}
	artifactKeys["source/final-image"]=sourceKey; hashes["source/final-image"]=sha256Hex(source)

	type sourceAsset struct {
		ID uuid.UUID
		StorageKey, MIMEType, Checksum string
		Size int64
		Width, Height int
		CreatedAt time.Time
	}
	rows,err:=h.db.QueryContext(ctx,`SELECT DISTINCT a.id,a.storage_key,a.mime_type,a.checksum,a.size,a.width,a.height,a.created_at
		FROM assets a JOIN "references" r ON r.asset_id=a.id
		WHERE r.project_id=$1 AND r.user_id=$2 AND a.lifecycle_status='active'
		ORDER BY a.created_at ASC,a.id ASC`,projectID,userID)
	if err!=nil{return fmt.Errorf("load source assets: %w",err)}
	defer func() { _ = rows.Close() }()
	var sourceAssets []sourceAsset
	for rows.Next(){var x sourceAsset;if err:=rows.Scan(&x.ID,&x.StorageKey,&x.MIMEType,&x.Checksum,&x.Size,&x.Width,&x.Height,&x.CreatedAt);err!=nil{return fmt.Errorf("scan source asset: %w",err)};sourceAssets=append(sourceAssets,x)}
	if err:=rows.Err();err!=nil{return fmt.Errorf("load source assets: %w",err)}

	// Export every active mask asset belonging to the project. Masks are regular assets with type=mask.
	maskRows,err:=h.db.QueryContext(ctx,`SELECT id,storage_key,mime_type,size FROM assets WHERE project_id=$1 AND user_id=$2 AND lifecycle_status='active' AND type='mask' ORDER BY created_at ASC,id ASC`,projectID,userID)
	if err!=nil{return fmt.Errorf("load mask assets: %w",err)}
	for maskRows.Next(){
		var id uuid.UUID; var key,mime string; var expectedSize int64
		if err:=maskRows.Scan(&id,&key,&mime,&expectedSize);err!=nil{_ = maskRows.Close();return fmt.Errorf("scan mask asset: %w",err)}
		obj,_,err:=h.storage.Get(ctx,key);if err!=nil{_ = maskRows.Close();return fmt.Errorf("read mask asset %s: %w",id,err)}
		data,readErr:=io.ReadAll(io.LimitReader(obj,assets.MaxAssetSize+1));_ = obj.Close()
		if readErr!=nil{_ = maskRows.Close();return fmt.Errorf("read mask asset %s: %w",id,readErr)}
		if int64(len(data))>assets.MaxAssetSize{_ = maskRows.Close();return fmt.Errorf("mask asset %s exceeds export limit",id)}
		if expectedSize > 0 && int64(len(data)) != expectedSize{_ = maskRows.Close();return fmt.Errorf("mask asset %s size mismatch",id)}
		outKey:=fmt.Sprintf("projects/%s/exports/%s/masks/%s",projectID,exportID,id)
		if err:=h.storage.Put(ctx,outKey,strings.NewReader(string(data)),int64(len(data)),storage.PutOptions{ContentType:mime});err!=nil{_ = maskRows.Close();return fmt.Errorf("write mask asset %s: %w",id,err)}
		name:=maskArtifactName(id);artifactKeys[name]=outKey;hashes[name]=sha256Hex(data)
	}
	if err:=maskRows.Err();err!=nil{_ = maskRows.Close();return fmt.Errorf("load mask assets: %w",err)}
	_ = maskRows.Close()

	for _,src:=range sourceAssets {
		obj,_,err:=h.storage.Get(ctx,src.StorageKey);if err!=nil{return fmt.Errorf("read source asset %s: %w",src.ID,err)}
		data,readErr:=io.ReadAll(io.LimitReader(obj,assets.MaxAssetSize+1));_ = obj.Close();if readErr!=nil{return fmt.Errorf("read source asset %s: %w",src.ID,readErr)}
		if int64(len(data))>assets.MaxAssetSize{return fmt.Errorf("source asset %s exceeds export limit",src.ID)}
		key:=fmt.Sprintf("projects/%s/exports/%s/source/%s",projectID,exportID,src.ID)
		if err:=h.storage.Put(ctx,key,strings.NewReader(string(data)),int64(len(data)),storage.PutOptions{ContentType:src.MIMEType});err!=nil{return fmt.Errorf("write source asset %s: %w",src.ID,err)}
		name:="source/"+src.ID.String();artifactKeys[name]=key;hashes[name]=sha256Hex(data)
	}

	pdfBytes:=buildPDFReport(reportMeta, verification)
	pdfKey:=fmt.Sprintf("projects/%s/exports/%s/creation-report.pdf",projectID,exportID)
	if err:=h.storage.Put(ctx,pdfKey,strings.NewReader(string(pdfBytes)),int64(len(pdfBytes)),storage.PutOptions{ContentType:"application/pdf"});err!=nil{return err}
	artifactKeys["creation-report.pdf"]=pdfKey;hashes["creation-report.pdf"]=sha256Hex(pdfBytes)

	manifestArtifacts:=map[string]string{}
	for k,v:=range artifactKeys{manifestArtifacts[k]=v}
	manifestHashes:=map[string]string{}
	for k,v:=range hashes{manifestHashes[k]=v}
	manifest["artifacts"]=manifestArtifacts
	manifest["hashes"]=manifestHashes
	manifestBytes,_:=json.MarshalIndent(manifest,"","  ")
	manifestKey:=fmt.Sprintf("projects/%s/exports/%s/manifest.json",projectID,exportID)
	if err:=h.storage.Put(ctx,manifestKey,strings.NewReader(string(manifestBytes)),int64(len(manifestBytes)),storage.PutOptions{ContentType:"application/json"});err!=nil{return err}
	artifactKeys["manifest.json"]=manifestKey;hashes["manifest.json"]=sha256Hex(manifestBytes)

	keys:=make([]string,0,len(hashes));for k:=range hashes{keys=append(keys,k)};sort.Strings(keys)
	var hashText strings.Builder
	for _,k:=range keys{fmt.Fprintf(&hashText,"%s  %s\n",hashes[k],k)}
	hashBytes:=[]byte(hashText.String())
	hashKey:=fmt.Sprintf("projects/%s/exports/%s/hashes.txt",projectID,exportID)
	if err:=h.storage.Put(ctx,hashKey,strings.NewReader(hashText.String()),int64(len(hashBytes)),storage.PutOptions{ContentType:"text/plain"});err!=nil{return err}
	artifactKeys["hashes.txt"]=hashKey

	artifactsJSON,_:=json.Marshal(artifactKeys)
	manifestJSON,_:=json.Marshal(manifest)
	_,err=h.db.ExecContext(ctx,`UPDATE export_records SET status='completed',artifacts=$2,manifest=$3,completed_at=now(),error=NULL WHERE id=$1`,exportID,artifactsJSON,manifestJSON)
	return err
}

func (h *Handler) get(w http.ResponseWriter,r *http.Request) {
	p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return}
	id,err:=uuid.Parse(r.PathValue("export_id"));if err!=nil{errJSON(w,400,"invalid_export_id","invalid export id");return}
	out,err:=h.load(r.Context(),p.UserID,id);if errors.Is(err,sql.ErrNoRows){errJSON(w,404,"export_not_found","export not found");return};if err!=nil{errJSON(w,500,"export_load_failed","could not load export");return}
	type Signed struct{URL string `json:"url"`;ExpiresAt time.Time `json:"expires_at"`}
	signed:=map[string]Signed{}
	for name,key:=range out.Artifacts{u,err:=h.storage.PresignGet(r.Context(),key,10*time.Minute);if err!=nil{errJSON(w,503,"export_url_failed","could not create export URL");return};signed[name]=Signed{u.URL,u.ExpiresAt}}
	out.Artifacts=map[string]string{}
	for name,v:=range signed{out.Artifacts[name]=v.URL;_ = v}
	writeJSON(w,200,out)
}

func (h *Handler) legacy(w http.ResponseWriter,r *http.Request){
	r2:=r.Clone(r.Context())
	r2.URL.Path=strings.TrimSuffix(r.URL.Path,"/assets/"+r.PathValue("asset_id")+"/export")+"/exports"
	body,_:=json.Marshal(map[string]any{"final_asset_id":r.PathValue("asset_id")})
	r2.Body=io.NopCloser(strings.NewReader(string(body)));r2.ContentLength=int64(len(body))
	h.create(w,r2)
}

func (h *Handler) load(ctx context.Context,userID,id uuid.UUID)(Export,error){
	var out Export;var a,m []byte
	err:=h.db.QueryRowContext(ctx,`SELECT e.id,e.project_id,e.user_id,e.final_asset_id,e.final_iteration_id,e.status,e.artifacts,e.manifest,e.error,e.created_at,e.completed_at FROM export_records e JOIN projects p ON p.id=e.project_id WHERE e.id=$1 AND e.user_id=$2 AND p.user_id=$2 AND p.status <> 'deleted'`,id,userID).Scan(&out.ID,&out.ProjectID,&out.UserID,&out.FinalAssetID,&out.FinalIterationID,&out.Status,&a,&m,&out.Error,&out.CreatedAt,&out.CompletedAt)
	if err!=nil{return out,err};_ = json.Unmarshal(a,&out.Artifacts);_ = json.Unmarshal(m,&out.Manifest);return out,nil
}
func(h *Handler)loadJSON(ctx context.Context,q string,args ...any)([]byte,error){var raw []byte;if err:=h.db.QueryRowContext(ctx,q,args...).Scan(&raw);err!=nil{return nil,err};if len(raw)==0{return []byte("[]"),nil};return raw,nil}
func(h *Handler)writeAction(r *http.Request,userID,projectID,exportID uuid.UUID,payload map[string]any){if h.events!=nil{_,_=h.events.Append(r.Context(),userID,projectID,"export.completed","export",exportID,payload)};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:userID,ProjectID:projectID,EntityType:"export",EntityID:exportID,Action:"export.completed",Payload:payload});if h.events!=nil{_,_=h.events.Append(r.Context(),userID,projectID,"provenance.updated","export",exportID,map[string]any{"action":"export.completed"})}};if h.actions!=nil{_,_=h.actions.Create(r.Context(),userID,projectID,humanactions.Request{ActionType:"EXPORT_CREATED",Payload:payload,NewState:payload})}}
func sha256Hex(data []byte)string{s:=sha256.Sum256(data);return hex.EncodeToString(s[:])}
func countJSON(raw []byte)int{var v []any;if json.Unmarshal(raw,&v)!=nil{return 0};return len(v)}
func decodeJSON(r *http.Request,v any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();return d.Decode(v)}
func buildPDFReport(meta map[string]any,verification provenance.Verification)[]byte{title:="Creation Report";lines:=[]string{title,fmt.Sprintf("Project: %v",meta["project_id"]),fmt.Sprintf("Export: %v",meta["export_id"]),fmt.Sprintf("Final asset: %v",meta["final_asset_id"]),fmt.Sprintf("Generated: %v",meta["generated_at"]),fmt.Sprintf("Provenance valid: %t; events: %d",verification.Valid,verification.EventsChecked),fmt.Sprintf("References: %v; prompts: %v; actions: %v",meta["reference_count"],meta["prompt_count"],meta["human_action_count"]),"Similarity analysis is informational, not a legal conclusion."};return minimalPDF(lines)}
func minimalPDF(lines []string) []byte {
	var b strings.Builder
	offsets := []int{}
	b.WriteString("%PDF-1.4\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	content := "BT /F1 11 Tf 50 760 Td "
	for _, line := range lines {
		line = strings.ReplaceAll(line, "\\", "\\\\")
		line = strings.ReplaceAll(line, "(", "\\(")
		line = strings.ReplaceAll(line, ")", "\\)")
		content += "(" + line + ") Tj 0 -18 Td "
	}
	content += "ET"
	objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	for i, object := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xref)
	return []byte(b.String())
}

func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func errJSON(w http.ResponseWriter,status int,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}

func maskArtifactName(id uuid.UUID) string { return "mask/" + id.String() }
