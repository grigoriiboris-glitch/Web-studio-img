package rights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type RegistryItem struct {
	ID uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	ProjectID uuid.UUID `json:"project_id"`
	TargetType string `json:"target_type"`
	TargetID uuid.UUID `json:"target_id"`
	Ownership string `json:"ownership"`
	License string `json:"license"`
	LicenseSource *string `json:"license_source,omitempty"`
	VerificationState string `json:"verification_state"`
	VerificationDate *time.Time `json:"verification_date,omitempty"`
	Notes *string `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Constraint struct {
	ID uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	ProjectID *uuid.UUID `json:"project_id,omitempty"`
	Kind string `json:"kind"`
	Value string `json:"value"`
	Active bool `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Handler struct{db *sql.DB;actions *humanactions.Store;provenance *provenance.Store}
func NewHandler(db *sql.DB,a *humanactions.Store,p *provenance.Store)(*Handler,error){if db==nil{return nil,errors.New("rights handler requires database")};return &Handler{db:db,actions:a,provenance:p},nil}
func(h *Handler)Register(mux *http.ServeMux){
	mux.HandleFunc("GET /api/v1/projects/{project_id}/rights",h.listRights)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/rights",h.createRight)
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}/rights/{right_id}",h.updateRight)
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}/rights/{right_id}",h.deleteRight)
	mux.HandleFunc("GET /api/v1/constraints",h.listGlobalConstraints)
	mux.HandleFunc("POST /api/v1/constraints",h.createGlobalConstraint)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/constraints",h.listProjectConstraints)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/constraints",h.createProjectConstraint)
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}/constraints/{constraint_id}",h.updateConstraint)
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}/constraints/{constraint_id}",h.deleteConstraint)
}
type rightInput struct{TargetType string `json:"target_type"`;TargetID uuid.UUID `json:"target_id"`;Ownership string `json:"ownership"`;License string `json:"license"`;LicenseSource *string `json:"license_source,omitempty"`;VerificationState string `json:"verification_state"`;VerificationDate *time.Time `json:"verification_date,omitempty"`;Notes *string `json:"notes,omitempty"`}
type constraintInput struct{Kind string `json:"kind"`;Value string `json:"value"`;Active *bool `json:"active,omitempty"`}
func uid(r *http.Request)(uuid.UUID,bool){p,ok:=auth.PrincipalFromContext(r.Context());if !ok{return uuid.Nil,false};return p.UserID,true}
func decode(r *http.Request,v any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return errors.New("multiple json values")};return nil}
func(h *Handler)ownedProject(ctx context.Context,u,p uuid.UUID)bool{var ok bool;return h.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`,p,u).Scan(&ok)==nil&&ok}
func(h *Handler)targetOwned(ctx context.Context,u,p uuid.UUID,t string,id uuid.UUID)bool{var q string;if t=="asset"{q=`SELECT EXISTS(SELECT 1 FROM assets a JOIN projects p ON p.id=a.project_id WHERE a.id=$1 AND a.project_id=$2 AND p.user_id=$3)`}else if t=="reference"{q=`SELECT EXISTS(SELECT 1 FROM "references" r JOIN projects p ON p.id=r.project_id WHERE r.id=$1 AND r.project_id=$2 AND p.user_id=$3)`}else{return false};var ok bool;return h.db.QueryRowContext(ctx,q,id,p,u).Scan(&ok)==nil&&ok}

func(h *Handler)listRights(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};rows,e:=h.db.QueryContext(r.Context(),`SELECT id,user_id,project_id,target_type,target_id,ownership,license,license_source,verification_state,verification_date,notes,created_at,updated_at FROM rights_registry WHERE user_id=$1 AND project_id=$2 ORDER BY updated_at DESC`,u,p);if e!=nil{err(w,500,"rights_list_failed","could not list rights");return};defer rows.Close();out:=[]RegistryItem{};for rows.Next(){var x RegistryItem;if e:=rows.Scan(&x.ID,&x.UserID,&x.ProjectID,&x.TargetType,&x.TargetID,&x.Ownership,&x.License,&x.LicenseSource,&x.VerificationState,&x.VerificationDate,&x.Notes,&x.CreatedAt,&x.UpdatedAt);e!=nil{err(w,500,"rights_list_failed","could not read rights");return};out=append(out,x)};write(w,200,map[string]any{"items":out})}
func(h *Handler)createRight(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};var in rightInput;if e=decode(r,&in);e!=nil{err(w,400,"invalid_request","invalid rights payload");return};in.TargetType=strings.ToLower(strings.TrimSpace(in.TargetType));in.Ownership=strings.ToLower(strings.TrimSpace(in.Ownership));in.License=strings.TrimSpace(in.License);in.VerificationState=strings.ToLower(strings.TrimSpace(in.VerificationState));if in.TargetID==uuid.Nil||!h.targetOwned(r.Context(),u,p,in.TargetType,in.TargetID){err(w,400,"invalid_target","target is not owned by this project");return};if in.Ownership==""{in.Ownership="unknown"};if in.License==""{in.License="unknown"};if in.VerificationState==""{in.VerificationState="unknown"};var x RegistryItem;e=h.db.QueryRowContext(r.Context(),`INSERT INTO rights_registry(user_id,project_id,target_type,target_id,ownership,license,license_source,verification_state,verification_date,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(user_id,target_type,target_id) DO UPDATE SET project_id=EXCLUDED.project_id,ownership=EXCLUDED.ownership,license=EXCLUDED.license,license_source=EXCLUDED.license_source,verification_state=EXCLUDED.verification_state,verification_date=EXCLUDED.verification_date,notes=EXCLUDED.notes,updated_at=now() RETURNING id,user_id,project_id,target_type,target_id,ownership,license,license_source,verification_state,verification_date,notes,created_at,updated_at`,u,p,in.TargetType,in.TargetID,in.Ownership,in.License,in.LicenseSource,in.VerificationState,in.VerificationDate,in.Notes).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.TargetType,&x.TargetID,&x.Ownership,&x.License,&x.LicenseSource,&x.VerificationState,&x.VerificationDate,&x.Notes,&x.CreatedAt,&x.UpdatedAt);if e!=nil{err(w,500,"rights_create_failed","could not save rights metadata");return};h.audit(r,u,p,x.ID,"RIGHTS_UPDATED",map[string]any{"target_type":x.TargetType,"target_id":x.TargetID,"verification_state":x.VerificationState});write(w,201,x)}
func(h *Handler)updateRight(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};id,e:=uuid.Parse(r.PathValue("right_id"));if e!=nil{err(w,400,"invalid_right_id","invalid right id");return};var in rightInput;if e=decode(r,&in);e!=nil{err(w,400,"invalid_request","invalid rights payload");return};var x RegistryItem;e=h.db.QueryRowContext(r.Context(),`UPDATE rights_registry SET ownership=COALESCE(NULLIF($1,''),ownership),license=COALESCE(NULLIF($2,''),license),license_source=COALESCE($3,license_source),verification_state=COALESCE(NULLIF($4,''),verification_state),verification_date=COALESCE($5,verification_date),notes=COALESCE($6,notes),updated_at=now() WHERE id=$7 AND user_id=$8 AND project_id=$9 RETURNING id,user_id,project_id,target_type,target_id,ownership,license,license_source,verification_state,verification_date,notes,created_at,updated_at`,in.Ownership,in.License,in.LicenseSource,in.VerificationState,in.VerificationDate,in.Notes,id,u,p).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.TargetType,&x.TargetID,&x.Ownership,&x.License,&x.LicenseSource,&x.VerificationState,&x.VerificationDate,&x.Notes,&x.CreatedAt,&x.UpdatedAt);if errors.Is(e,sql.ErrNoRows){err(w,404,"rights_not_found","rights record not found");return};if e!=nil{err(w,500,"rights_update_failed","could not update rights metadata");return};h.audit(r,u,p,x.ID,"RIGHTS_UPDATED",map[string]any{"target_type":x.TargetType,"target_id":x.TargetID,"verification_state":x.VerificationState});write(w,200,x)}
func(h *Handler)deleteRight(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};id,e:=uuid.Parse(r.PathValue("right_id"));if e!=nil{err(w,400,"invalid_right_id","invalid right id");return};res,e:=h.db.ExecContext(r.Context(),`DELETE FROM rights_registry WHERE id=$1 AND user_id=$2 AND project_id=$3`,id,u,p);if e!=nil{err(w,500,"rights_delete_failed","could not delete rights record");return};n,_:=res.RowsAffected();if n!=1{err(w,404,"rights_not_found","rights record not found");return};h.audit(r,u,p,id,"RIGHTS_UPDATED",map[string]any{"deleted":true});w.WriteHeader(http.StatusNoContent)}

func(h *Handler)listGlobalConstraints(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};h.listConstraints(w,r,u,nil)}
func(h *Handler)listProjectConstraints(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};h.listConstraints(w,r,u,&p)}
func(h *Handler)listConstraints(w http.ResponseWriter,r *http.Request,u uuid.UUID,pid *uuid.UUID){var rows *sql.Rows;var e error;if pid==nil{rows,e=h.db.QueryContext(r.Context(),`SELECT id,user_id,project_id,kind,value,active,created_at,updated_at FROM do_not_use_constraints WHERE user_id=$1 AND project_id IS NULL ORDER BY created_at DESC`,u)}else{rows,e=h.db.QueryContext(r.Context(),`SELECT id,user_id,project_id,kind,value,active,created_at,updated_at FROM do_not_use_constraints WHERE user_id=$1 AND (project_id IS NULL OR project_id=$2) ORDER BY project_id NULLS FIRST,created_at DESC`,u,*pid)};if e!=nil{err(w,500,"constraints_list_failed","could not list constraints");return};defer rows.Close();out:=[]Constraint{};for rows.Next(){var x Constraint;if e:=rows.Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Kind,&x.Value,&x.Active,&x.CreatedAt,&x.UpdatedAt);e!=nil{err(w,500,"constraints_list_failed","could not read constraints");return};out=append(out,x)};write(w,200,map[string]any{"constraints":out})}
func(h *Handler)createGlobalConstraint(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};h.createConstraint(w,r,u,nil)}
func(h *Handler)createProjectConstraint(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};h.createConstraint(w,r,u,&p)}
func(h *Handler)createConstraint(w http.ResponseWriter,r *http.Request,u uuid.UUID,pid *uuid.UUID){var in constraintInput;if e:=decode(r,&in);e!=nil{err(w,400,"invalid_request","invalid constraint payload");return};in.Kind=strings.ToLower(strings.TrimSpace(in.Kind));in.Value=strings.TrimSpace(in.Value);if in.Value==""||len([]rune(in.Value))>500||in.Kind==""{err(w,400,"invalid_constraint","kind and value are required");return};active:=true;if in.Active!=nil{active=*in.Active};var x Constraint;var e error;if pid==nil{e=h.db.QueryRowContext(r.Context(),`INSERT INTO do_not_use_constraints(user_id,project_id,kind,value,active) VALUES($1,NULL,$2,$3,$4) RETURNING id,user_id,project_id,kind,value,active,created_at,updated_at`,u,in.Kind,in.Value,active).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Kind,&x.Value,&x.Active,&x.CreatedAt,&x.UpdatedAt)}else{e=h.db.QueryRowContext(r.Context(),`INSERT INTO do_not_use_constraints(user_id,project_id,kind,value,active) VALUES($1,$2,$3,$4,$5) RETURNING id,user_id,project_id,kind,value,active,created_at,updated_at`,u,*pid,in.Kind,in.Value,active).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Kind,&x.Value,&x.Active,&x.CreatedAt,&x.UpdatedAt)};if e!=nil{err(w,500,"constraint_create_failed","could not create constraint");return};if pid!=nil{h.audit(r,u,*pid,x.ID,"DO_NOT_USE_UPDATED",map[string]any{"kind":x.Kind,"value":x.Value,"active":x.Active})};write(w,201,x)}
func(h *Handler)updateConstraint(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};id,e:=uuid.Parse(r.PathValue("constraint_id"));if e!=nil{err(w,400,"invalid_constraint_id","invalid constraint id");return};var in constraintInput;if e=decode(r,&in);e!=nil{err(w,400,"invalid_request","invalid constraint payload");return};active:=true;if in.Active!=nil{active=*in.Active};var x Constraint;e=h.db.QueryRowContext(r.Context(),`UPDATE do_not_use_constraints SET kind=COALESCE(NULLIF($1,''),kind),value=COALESCE(NULLIF($2,''),value),active=$3,updated_at=now() WHERE id=$4 AND user_id=$5 AND (project_id=$6 OR project_id IS NULL) RETURNING id,user_id,project_id,kind,value,active,created_at,updated_at`,strings.ToLower(strings.TrimSpace(in.Kind)),strings.TrimSpace(in.Value),active,id,u,p).Scan(&x.ID,&x.UserID,&x.ProjectID,&x.Kind,&x.Value,&x.Active,&x.CreatedAt,&x.UpdatedAt);if errors.Is(e,sql.ErrNoRows){err(w,404,"constraint_not_found","constraint not found");return};if e!=nil{err(w,500,"constraint_update_failed","could not update constraint");return};h.audit(r,u,p,x.ID,"DO_NOT_USE_UPDATED",map[string]any{"kind":x.Kind,"value":x.Value,"active":x.Active});write(w,200,x)}
func(h *Handler)deleteConstraint(w http.ResponseWriter,r *http.Request){u,ok:=uid(r);if !ok{err(w,401,"unauthorized","authentication required");return};p,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil||!h.ownedProject(r.Context(),u,p){err(w,404,"project_not_found","project not found");return};id,e:=uuid.Parse(r.PathValue("constraint_id"));if e!=nil{err(w,400,"invalid_constraint_id","invalid constraint id");return};res,e:=h.db.ExecContext(r.Context(),`DELETE FROM do_not_use_constraints WHERE id=$1 AND user_id=$2 AND (project_id=$3 OR project_id IS NULL)`,id,u,p);if e!=nil{err(w,500,"constraint_delete_failed","could not delete constraint");return};n,_:=res.RowsAffected();if n!=1{err(w,404,"constraint_not_found","constraint not found");return};h.audit(r,u,p,id,"DO_NOT_USE_UPDATED",map[string]any{"deleted":true});w.WriteHeader(http.StatusNoContent)}
func(h *Handler)audit(r *http.Request,u,p,id uuid.UUID,a string,payload map[string]any){if h.actions!=nil{_,_=h.actions.Create(r.Context(),u,p,humanactions.Request{ActionType:a,Payload:payload})};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:p,EntityType:"rights",EntityID:id,Action:strings.ToLower(a),Payload:payload})}}
func write(w http.ResponseWriter,s int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(s);_=json.NewEncoder(w).Encode(v)}
func err(w http.ResponseWriter,s int,c,m string){write(w,s,map[string]any{"error":map[string]string{"code":c,"message":m,"request_id":uuid.NewString()}})}
