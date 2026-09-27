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

type Handler struct{ db *sql.DB }
func NewHandler(db *sql.DB)(*Handler,error){if db==nil{return nil,errors.New("library handler requires database")};return &Handler{db:db},nil}
func(h *Handler)Register(mux *http.ServeMux){
	mux.HandleFunc("GET /api/v1/projects/{project_id}/materials",func(w http.ResponseWriter,r *http.Request){h.list(w,r,"material")})
	mux.HandleFunc("POST /api/v1/projects/{project_id}/materials",func(w http.ResponseWriter,r *http.Request){h.create(w,r,"material")})
	mux.HandleFunc("GET /api/v1/projects/{project_id}/textures",func(w http.ResponseWriter,r *http.Request){h.list(w,r,"texture")})
	mux.HandleFunc("POST /api/v1/projects/{project_id}/textures",func(w http.ResponseWriter,r *http.Request){h.create(w,r,"texture")})
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
	rows,e:=h.db.QueryContext(r.Context(),query,args...);if e!=nil{writeErr(w,500,"library_list_failed","could not list library items");return};defer rows.Close()
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
	e=h.db.QueryRowContext(r.Context(),`INSERT INTO library_items(user_id,kind,category,name,description,tags,prompt_fragment,preview_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,user_id,kind,category,name,description,tags,prompt_fragment,preview_key,created_at,updated_at`,u.UserID,kind,in.Category,in.Name,in.Description,raw,in.PromptFragment,in.PreviewKey).Scan(&x.ID,&x.UserID,&x.Kind,&x.Category,&x.Name,&x.Description,&stored,&x.PromptFragment,&x.PreviewKey,&x.CreatedAt,&x.UpdatedAt)
	if e!=nil{writeErr(w,500,"library_create_failed","could not create library item");return};_=json.Unmarshal(stored,&x.Tags);writeJSON(w,201,x)
}
func writeJSON(w http.ResponseWriter,s int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(s);_=json.NewEncoder(w).Encode(v)}
func writeErr(w http.ResponseWriter,s int,c,m string){writeJSON(w,s,map[string]any{"error":map[string]string{"code":c,"message":m,"request_id":uuid.NewString()})}
