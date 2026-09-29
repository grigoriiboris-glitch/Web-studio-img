package manualedits

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

var (
	ErrInvalid = errors.New("invalid manual edit")
	ErrNotFound = errors.New("manual edit not found")
	ErrImmutable = errors.New("manual edit is immutable")
)

type Operation string
const (
	OperationPaint Operation = "paint"
	OperationErase Operation = "erase"
	OperationMask Operation = "mask"
	OperationComposite Operation = "composite"
)
type Status string
const (
	StatusDraft Status = "draft"
	StatusApplied Status = "applied"
	StatusRejected Status = "rejected"
)

type Edit struct {
	ID uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	UserID uuid.UUID `json:"user_id"`
	IterationID *uuid.UUID `json:"iteration_id,omitempty"`
	SourceAssetID uuid.UUID `json:"source_asset_id"`
	MaskAssetID *uuid.UUID `json:"mask_asset_id,omitempty"`
	ResultAssetID *uuid.UUID `json:"result_asset_id,omitempty"`
	Operation Operation `json:"operation"`
	Prompt *string `json:"prompt,omitempty"`
	Parameters map[string]any `json:"parameters"`
	Status Status `json:"status"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct { db *sql.DB }

func NewStore(db *sql.DB) (*Store,error) {
	if db == nil { return nil, errors.New("manual edit store requires database") }
	return &Store{db:db},nil
}

func (s *Store) Create(ctx context.Context, userID, projectID uuid.UUID, iterationID, sourceID, maskID *uuid.UUID, operation Operation, prompt *string, parameters map[string]any, key *string) (Edit,error) {
	if sourceID == nil || !validOperation(operation) { return Edit{},ErrInvalid }
	if (operation == OperationErase || operation == OperationMask) && maskID == nil { return Edit{},ErrInvalid }
	if parameters == nil { parameters=map[string]any{} }
	raw,err:=json.Marshal(parameters); if err!=nil{return Edit{},fmt.Errorf("encode parameters: %w",err)}
	if key != nil && strings.TrimSpace(*key)=="" { key=nil }
	var e Edit
	err=s.db.QueryRowContext(ctx,
		"INSERT INTO manual_edits(project_id,user_id,source_asset_id,mask_asset_id,operation,prompt,parameters,idempotency_key) "+
		"SELECT $1,$2,$3,$4,$5,$6,$7,$8 "+
		"WHERE EXISTS (SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted') "+
		"AND EXISTS (SELECT 1 FROM assets WHERE id=$3 AND project_id=$1) "+
		"AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM assets WHERE id=$4 AND project_id=$1)) "+
		"ON CONFLICT (user_id,project_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO UPDATE SET id=manual_edits.id "+
		"RETURNING id,project_id,user_id,iteration_id,source_asset_id,mask_asset_id,result_asset_id,operation,prompt,parameters,status,idempotency_key,created_at,updated_at",
		projectID,userID,iterationID,*sourceID,maskID,operation,prompt,raw,key).Scan(&e.ID,&e.ProjectID,&e.UserID,&e.IterationID,&e.SourceAssetID,&e.MaskAssetID,&e.ResultAssetID,&e.Operation,&e.Prompt,&raw,&e.Status,&e.IdempotencyKey,&e.CreatedAt,&e.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){return Edit{},ErrNotFound}
	if err!=nil{return Edit{},fmt.Errorf("create manual edit: %w",err)}
	if err=json.Unmarshal(raw,&e.Parameters);err!=nil{return Edit{},err}
	return e,nil
}

func (s *Store) Get(ctx context.Context,userID,id uuid.UUID)(Edit,error){
	return s.get(ctx,userID,id)
}

func (s *Store) List(ctx context.Context,userID,projectID uuid.UUID)([]Edit,error){
	rows,err:=s.db.QueryContext(ctx,"SELECT id,project_id,user_id,iteration_id,source_asset_id,mask_asset_id,result_asset_id,operation,prompt,parameters,status,idempotency_key,created_at,updated_at FROM manual_edits WHERE user_id=$1 AND project_id=$2 ORDER BY created_at ASC,id ASC",userID,projectID)
	if err!=nil{return nil,err};defer rows.Close()
	var out []Edit
	for rows.Next(){e,err:=scan(rows);if err!=nil{return nil,err};out=append(out,e)}
	return out,rows.Err()
}

func (s *Store) Apply(ctx context.Context,userID,id,resultID uuid.UUID,title *string)(Edit,error){
	tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return Edit{},err};defer tx.Rollback()
	var e Edit;var raw []byte
	err=tx.QueryRowContext(ctx,"SELECT id,project_id,user_id,iteration_id,source_asset_id,mask_asset_id,result_asset_id,operation,prompt,parameters,status,idempotency_key,created_at,updated_at FROM manual_edits WHERE id=$1 AND user_id=$2 FOR UPDATE",id,userID).Scan(&e.ID,&e.ProjectID,&e.UserID,&e.IterationID,&e.SourceAssetID,&e.MaskAssetID,&e.ResultAssetID,&e.Operation,&e.Prompt,&raw,&e.Status,&e.IdempotencyKey,&e.CreatedAt,&e.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){return Edit{},ErrNotFound};if err!=nil{return Edit{},err}
	if err=json.Unmarshal(raw,&e.Parameters);err!=nil{return Edit{},err}
	if e.Status!=StatusDraft{return Edit{},ErrImmutable}
	var exists bool
	if err=tx.QueryRowContext(ctx,"SELECT EXISTS(SELECT 1 FROM assets WHERE id=$1 AND project_id=$2)",resultID,e.ProjectID).Scan(&exists);err!=nil{return Edit{},err};if !exists{return Edit{},ErrNotFound}
	parent:=e.IterationID
	if parent!=nil{if err=tx.QueryRowContext(ctx,"SELECT EXISTS(SELECT 1 FROM iterations WHERE id=$1 AND project_id=$2)",*parent,e.ProjectID).Scan(&exists);err!=nil{return Edit{},err};if !exists{return Edit{},ErrNotFound}}
	if title==nil {v:="Manual edit";title=&v}
	var iterationID uuid.UUID
	if err=tx.QueryRowContext(ctx,"INSERT INTO iterations(project_id,parent_iteration_id,type,title,description) VALUES($1,$2,'manual_edit',$3,$4) RETURNING id",e.ProjectID,parent,title,effectiveDescription(e.Operation,e.Prompt)).Scan(&iterationID);err!=nil{return Edit{},fmt.Errorf("create approved iteration: %w",err)}
	if _,err=tx.ExecContext(ctx,"UPDATE manual_edits SET result_asset_id=$1,iteration_id=$2,status='applied',updated_at=now() WHERE id=$3",resultID,iterationID,id);err!=nil{return Edit{},err}
	e.ResultAssetID=&resultID;e.IterationID=&iterationID;e.Status=StatusApplied;e.UpdatedAt=time.Now()
	if err=tx.Commit();err!=nil{return Edit{},err}
	return e,nil
}

func (s *Store) Reject(ctx context.Context,userID,id uuid.UUID)(Edit,error){
	var e Edit;var raw []byte
	err:=s.db.QueryRowContext(ctx,"UPDATE manual_edits SET status='rejected',updated_at=now() WHERE id=$1 AND user_id=$2 AND status='draft' RETURNING id,project_id,user_id,iteration_id,source_asset_id,mask_asset_id,result_asset_id,operation,prompt,parameters,status,idempotency_key,created_at,updated_at",id,userID).Scan(&e.ID,&e.ProjectID,&e.UserID,&e.IterationID,&e.SourceAssetID,&e.MaskAssetID,&e.ResultAssetID,&e.Operation,&e.Prompt,&raw,&e.Status,&e.IdempotencyKey,&e.CreatedAt,&e.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){return Edit{},ErrImmutable};if err!=nil{return Edit{},err};_ = json.Unmarshal(raw,&e.Parameters);return e,nil
}

func (s *Store) get(ctx context.Context,userID,id uuid.UUID)(Edit,error){
	var e Edit;var raw []byte
	err:=s.db.QueryRowContext(ctx,"SELECT id,project_id,user_id,iteration_id,source_asset_id,mask_asset_id,result_asset_id,operation,prompt,parameters,status,idempotency_key,created_at,updated_at FROM manual_edits WHERE id=$1 AND user_id=$2",id,userID).Scan(&e.ID,&e.ProjectID,&e.UserID,&e.IterationID,&e.SourceAssetID,&e.MaskAssetID,&e.ResultAssetID,&e.Operation,&e.Prompt,&raw,&e.Status,&e.IdempotencyKey,&e.CreatedAt,&e.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){return Edit{},ErrNotFound};if err!=nil{return Edit{},err};if err=json.Unmarshal(raw,&e.Parameters);err!=nil{return Edit{},err};return e,nil
}
func scan(r interface{Scan(...any)error})(Edit,error){var e Edit;var raw []byte;if err:=r.Scan(&e.ID,&e.ProjectID,&e.UserID,&e.IterationID,&e.SourceAssetID,&e.MaskAssetID,&e.ResultAssetID,&e.Operation,&e.Prompt,&raw,&e.Status,&e.IdempotencyKey,&e.CreatedAt,&e.UpdatedAt);err!=nil{return Edit{},err};if err:=json.Unmarshal(raw,&e.Parameters);err!=nil{return Edit{},err};return e,nil}
func validOperation(v Operation)bool{return v==OperationPaint||v==OperationErase||v==OperationMask||v==OperationComposite}
func effectiveDescription(op Operation,p *string)string{if p!=nil&&strings.TrimSpace(*p)!=""{return string(op)+": "+strings.TrimSpace(*p)};return string(op)}

type Handler struct{store *Store;events *events.Store;provenance *provenance.Store}
func NewHandler(store *Store,ev *events.Store,pv *provenance.Store)(*Handler,error){if store==nil{return nil,errors.New("manual edit handler requires store")};return &Handler{store:store,events:ev,provenance:pv},nil}
func(h *Handler)Register(mux *http.ServeMux){
	mux.HandleFunc("POST /api/v1/projects/{project_id}/manual-edits",h.create)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/manual-edits",h.list)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/manual-edits/{edit_id}",h.get)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/manual-edits/{edit_id}/apply",h.apply)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/manual-edits/{edit_id}/reject",h.reject)
}
func(h *Handler)create(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{writeErr(w,401,"unauthorized");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id");return};var in struct{IterationID *uuid.UUID `json:"iteration_id,omitempty"`;SourceAssetID uuid.UUID `json:"source_asset_id"`;MaskAssetID *uuid.UUID `json:"mask_asset_id,omitempty"`;Operation Operation `json:"operation"`;Prompt *string `json:"prompt,omitempty"`;Parameters map[string]any `json:"parameters,omitempty"`};if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{writeErr(w,400,"invalid_request");return};key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"));var kp *string;if key!=""{kp=&key};item,err:=h.store.Create(r.Context(),u,pid,in.IterationID,&in.SourceAssetID,in.MaskAssetID,in.Operation,in.Prompt,in.Parameters,kp);if errors.Is(err,ErrInvalid){writeErr(w,400,"invalid_manual_edit");return};if errors.Is(err,ErrNotFound){writeErr(w,404,"project_or_asset_not_found");return};if err!=nil{writeErr(w,500,"manual_edit_create_failed");return};h.emit(r,u,pid,item.ID,"manual_edit.created");writeJSON(w,201,item)}
func(h *Handler)list(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{writeErr(w,401,"unauthorized");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id");return};items,err:=h.store.List(r.Context(),u,pid);if err!=nil{writeErr(w,500,"manual_edit_list_failed");return};writeJSON(w,200,map[string]any{"manual_edits":items})}
func(h *Handler)get(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{writeErr(w,401,"unauthorized");return};id,err:=uuid.Parse(r.PathValue("edit_id"));if err!=nil{writeErr(w,400,"invalid_edit_id");return};item,err:=h.store.Get(r.Context(),u,id);if errors.Is(err,ErrNotFound){writeErr(w,404,"manual_edit_not_found");return};if err!=nil{writeErr(w,500,"manual_edit_get_failed");return};if item.ProjectID.String()!=r.PathValue("project_id"){writeErr(w,404,"manual_edit_not_found");return};writeJSON(w,200,item)}
func(h *Handler)apply(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{writeErr(w,401,"unauthorized");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id");return};id,err:=uuid.Parse(r.PathValue("edit_id"));if err!=nil{writeErr(w,400,"invalid_edit_id");return};var in struct{ResultAssetID uuid.UUID `json:"result_asset_id"`;Title *string `json:"title,omitempty"`};if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil||in.ResultAssetID==uuid.Nil{writeErr(w,400,"result_asset_id_required");return};item,err:=h.store.Apply(r.Context(),u,id,in.ResultAssetID,in.Title);if errors.Is(err,ErrNotFound){writeErr(w,404,"manual_edit_or_asset_not_found");return};if errors.Is(err,ErrImmutable){writeErr(w,409,"manual_edit_immutable");return};if err!=nil{writeErr(w,500,"manual_edit_apply_failed");return};if item.ProjectID!=pid{writeErr(w,404,"manual_edit_not_found");return};h.emit(r,u,pid,item.ID,"manual_edit.applied");writeJSON(w,200,item)}
func(h *Handler)reject(w http.ResponseWriter,r *http.Request){u,ok:=user(r);if !ok{writeErr(w,401,"unauthorized");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id");return};id,err:=uuid.Parse(r.PathValue("edit_id"));if err!=nil{writeErr(w,400,"invalid_edit_id");return};item,err:=h.store.Reject(r.Context(),u,id);if errors.Is(err,ErrImmutable){writeErr(w,409,"manual_edit_immutable");return};if err!=nil{writeErr(w,500,"manual_edit_reject_failed");return};if item.ProjectID!=pid{writeErr(w,404,"manual_edit_not_found");return};h.emit(r,u,pid,item.ID,"manual_edit.rejected");writeJSON(w,200,item)}
func(h *Handler)emit(r *http.Request,u,pid,id uuid.UUID,action string){if h.events!=nil{_,_=h.events.Append(r.Context(),u,pid,action,"manual_edit",id,map[string]any{"immutable":true})};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:pid,EntityType:"manual_edit",EntityID:id,Action:action})}}
func user(r *http.Request)(uuid.UUID,bool){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{return uuid.Nil,false};return p.UserID,true}
func writeErr(w http.ResponseWriter,status int,code string){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(map[string]any{"error":map[string]string{"code":code}})}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
