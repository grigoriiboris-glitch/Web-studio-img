package library

import (
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
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Item struct {
	ID uuid.UUID `json:"id"`
	UserID *uuid.UUID `json:"user_id,omitempty"`
	Kind string `json:"kind"`
	Category string `json:"category"`
	Name string `json:"name"`
	Description *string `json:"description,omitempty"`
	Tags []string `json:"tags"`
	PromptFragment string `json:"prompt_fragment"`
	PreviewKey *string `json:"preview_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Handler struct{ db *sql.DB; actions *humanactions.Store; provenance *provenance.Store }
func NewHandler(db *sql.DB)(*Handler,error){return NewHandlerWithAudit(db,nil,nil)}
func NewHandlerWithAudit(db *sql.DB,actions *humanactions.Store,provenanceStore *provenance.Store)(*Handler,error){if db==nil{return nil,errors.New("library handler requires database")};return &Handler{db:db,actions:actions,provenance:provenanceStore},nil}
func(h *Handler)Register(mux *http.ServeMux){
	mux.HandleFunc("GET /api/v1/projects/{project_id}/materials",func(w http.ResponseWriter,r *http.Request){h.list(w,r,"material")})
	mux.HandleFunc("POST /api/v1/projects/{project_id}/materials",func(w http.ResponseWriter,r *http.Request){h.create(w,r,"material")})
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}/materials/{item_id}",func(w http.ResponseWriter,r *http.Request){h.update(w,r,"material")})
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}/materials/{item_id}",func(w http.ResponseWriter,r *http.Request){h.remove(w,r,"material")})
	mux.HandleFunc("POST /api/v1/projects/{project_id}/materials/{item_id}/select",func(w http.ResponseWriter,r *http.Request){h.selectItem(w,r,"material")})
	mux.HandleFunc("GET /api/v1/projects/{project_id}/textures",func(w http.ResponseWriter,r *http.Request){h.list(w,r,"texture")})
	mux.HandleFunc("POST /api/v1/projects/{project_id}/textures",func(w http.ResponseWriter,r *http.Request){h.create(w,r,"texture")})
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}/textures/{item_id}",func(w http.ResponseWriter,r *http.Request){h.update(w,r,"texture")})
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}/textures/{item_id}",func(w http.ResponseWriter,r *http.Request){h.remove(w,r,"texture")})
	mux.HandleFunc("POST /api/v1/projects/{project_id}/textures/{item_id}/select",func(w http.ResponseWriter,r *http.Request){h.selectItem(w,r,"texture")})
}
func(h *Handler)list(w http.ResponseWriter,r *http.Request,kind string){
	u,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeErr(w,401,"unauthorized","authentication required");return}
	pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return}
	var exists bool;if err=h.db.QueryRowContext(r.Context(),`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`,pid,u.UserID).Scan(&exists);err!=nil||!exists{writeErr(w,404,"project_not_found","project not found");return}
	q:=strings.TrimSpace(r.URL.Query().Get("q"));category:=strings.TrimSpace(r.URL.Query().Get("category"))
	query:=`SELECT id,user_id,kind,category,name,description,tags,prompt_fragment,preview_key,created_at,updated_at FROM library_items WHERE kind=$1 AND (user_id IS NULL OR user_id=$2)`;args:=[]any{kind,u.UserID}
	if q!=""{query+=" AND (name ILIKE $3 OR COALESCE(description,'') ILIKE $3 OR prompt_fragment ILIKE $3)";args=append(args,"%"+q+"%")}
	if category!=""{query+=fmt.Sprintf(" AND category=$%d",len(args)+1);args=append(args,category)}
	query+=" ORDER BY user_id NULLS FIRST, created_at DESC LIMIT 200"
	rows,e:=h.db.QueryContext(r.Context(),query,args...);if e!=nil{writeErr(w,500,"library_list_failed","could not list library items");return};defer func() { _ = rows.Close() }()
	out:=[]Item{};for rows.Next(){var x Item;var tags []byte;if e:=rows.Scan(&x.ID,&x.UserID,&x.Kind,&x.Category,&x.Name,&x.Description,&tags,&x.PromptFragment,&x.PreviewKey,&x.CreatedAt,&x.UpdatedAt);e!=nil{writeErr(w,500,"library_list_failed","could not read library items");return};_=json.Unmarshal(tags,&x.Tags);out=append(out,x)}
	if e:=rows.Err();e!=nil{writeErr(w,500,"library_list_failed","could not read library items");return};writeJSON(w,200,map[string]any{"items":out})
}
func(h *Handler)create(w http.ResponseWriter,r *http.Request,kind string){
	u,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return}
	var exists bool;if err=h.db.QueryRowContext(r.Context(),`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`,pid,u.UserID).Scan(&exists);err!=nil||!exists{writeErr(w,404,"project_not_found","project not found");return}
	var in struct{Category string `json:"category"`;Name string `json:"name"`;Description *string `json:"description"`;Tags []string `json:"tags"`;PromptFragment string `json:"prompt_fragment"`;PreviewKey *string `json:"preview_key"`}
	d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if e:=d.Decode(&in);e!=nil{writeErr(w,400,"invalid_request","invalid library payload");return}
	in.Category=strings.TrimSpace(in.Category);in.Name=strings.TrimSpace(in.Name);in.PromptFragment=strings.TrimSpace(in.PromptFragment)
	if in.Category==""||in.Name==""||in.PromptFragment==""||len(in.Tags)>50||len([]rune(in.Name))>200||len([]rune(in.Category))>100||len([]rune(in.PromptFragment))>4000{writeErr(w,400,"invalid_library_item","invalid library item");return}
	raw,_:=json.Marshal(in.Tags);var x Item;var stored []byte
	err=h.db.QueryRowContext(r.Context(),`INSERT INTO library_items(user_id,kind,category,name,description,tags,prompt_fragment,preview_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,user_id,kind,category,name,description,tags,prompt_fragment,preview_key,created_at,updated_at`,u.UserID,kind,in.Category,in.Name,in.Description,raw,in.PromptFragment,in.PreviewKey).Scan(&x.ID,&x.UserID,&x.Kind,&x.Category,&x.Name,&x.Description,&stored,&x.PromptFragment,&x.PreviewKey,&x.CreatedAt,&x.UpdatedAt)
	if err!=nil{writeErr(w,500,"library_create_failed","could not create library item");return};_=json.Unmarshal(stored,&x.Tags);writeJSON(w,201,x)
}

func(h *Handler) update(w http.ResponseWriter,r *http.Request,kind string){
	u,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};id,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid item id");return}
	var in struct{Category string `json:"category"`;Name string `json:"name"`;Description *string `json:"description"`;Tags []string `json:"tags"`;PromptFragment string `json:"prompt_fragment"`;PreviewKey *string `json:"preview_key"`}
	d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if err:=d.Decode(&in);err!=nil{writeErr(w,400,"invalid_request","invalid library payload");return}
	in.Category=strings.TrimSpace(in.Category);in.Name=strings.TrimSpace(in.Name);in.PromptFragment=strings.TrimSpace(in.PromptFragment);if in.Category==""||in.Name==""||in.PromptFragment==""||len(in.Tags)>50||len([]rune(in.Name))>200||len([]rune(in.Category))>100||len([]rune(in.PromptFragment))>4000{writeErr(w,400,"invalid_library_item","invalid library item");return}
	raw,_:=json.Marshal(in.Tags);var x Item;var stored []byte
	err=h.db.QueryRowContext(r.Context(),`UPDATE library_items SET category=$1,name=$2,description=$3,tags=$4,prompt_fragment=$5,preview_key=$6,updated_at=now() WHERE id=$7 AND kind=$8 AND user_id=$9 RETURNING id,user_id,kind,category,name,description,tags,prompt_fragment,preview_key,created_at,updated_at`,in.Category,in.Name,in.Description,raw,in.PromptFragment,in.PreviewKey,id,kind,u.UserID).Scan(&x.ID,&x.UserID,&x.Kind,&x.Category,&x.Name,&x.Description,&stored,&x.PromptFragment,&x.PreviewKey,&x.CreatedAt,&x.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){writeErr(w,404,"library_item_not_found","user library item not found");return};if err!=nil{writeErr(w,500,"library_update_failed","could not update library item");return};_=json.Unmarshal(stored,&x.Tags);h.audit(r,u.UserID,pid,x.ID,strings.ToUpper(kind)+"_UPDATED",strings.ToLower(kind)+"_updated",map[string]any{"item_id":id,"kind":kind});writeJSON(w,200,x)
}
func(h *Handler) remove(w http.ResponseWriter,r *http.Request,kind string){
	u,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};id,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid item id");return}
	res,err:=h.db.ExecContext(r.Context(),`DELETE FROM library_items WHERE id=$1 AND kind=$2 AND user_id=$3`,id,kind,u.UserID);if err!=nil{writeErr(w,500,"library_delete_failed","could not delete library item");return};if n,_:=res.RowsAffected();n!=1{writeErr(w,404,"library_item_not_found","user library item not found");return};h.audit(r,u.UserID,pid,id,strings.ToUpper(kind)+"_DELETED",strings.ToLower(kind)+"_deleted",map[string]any{"item_id":id,"kind":kind});w.WriteHeader(http.StatusNoContent)
}
func(h *Handler) selectItem(w http.ResponseWriter,r *http.Request,kind string){
	u,ok:=auth.PrincipalFromContext(r.Context());if !ok{writeErr(w,401,"unauthorized","authentication required");return};pid,err:=uuid.Parse(r.PathValue("project_id"));if err!=nil{writeErr(w,400,"invalid_project_id","invalid project id");return};id,err:=uuid.Parse(r.PathValue("item_id"));if err!=nil{writeErr(w,400,"invalid_item_id","invalid item id");return}
	var x Item;var raw []byte;err=h.db.QueryRowContext(r.Context(),`SELECT id,user_id,kind,category,name,description,tags,prompt_fragment,preview_key,created_at,updated_at FROM library_items WHERE id=$1 AND kind=$2 AND (user_id IS NULL OR user_id=$3)`,id,kind,u.UserID).Scan(&x.ID,&x.UserID,&x.Kind,&x.Category,&x.Name,&x.Description,&raw,&x.PromptFragment,&x.PreviewKey,&x.CreatedAt,&x.UpdatedAt);if errors.Is(err,sql.ErrNoRows){writeErr(w,404,"library_item_not_found","library item not found");return};if err!=nil{writeErr(w,500,"library_select_failed","could not load library item");return};_=json.Unmarshal(raw,&x.Tags)
	h.audit(r,u.UserID,pid,id,strings.ToUpper(kind)+"_SELECTED",strings.ToLower(kind)+"_selected",map[string]any{"item_id":id,"kind":kind,"name":x.Name,"prompt_fragment":x.PromptFragment});writeJSON(w,200,map[string]any{"item":x,"selected":true})
}
func(h *Handler) audit(r *http.Request,u,pid,id uuid.UUID,action,provAction string,payload map[string]any){if h.actions!=nil{_,_=h.actions.Create(r.Context(),u,pid,humanactions.Request{ActionType:action,Payload:payload})};if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:u,ProjectID:pid,EntityType:"library_item",EntityID:id,Action:provAction,Payload:payload})}}
func writeJSON(w http.ResponseWriter,s int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(s);_=json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,s int,c,m string){writeJSON(w,s,map[string]any{"error":map[string]string{"code":c,"message":m,"request_id":uuid.NewString()}})}
