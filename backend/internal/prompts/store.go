
package prompts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"github.com/google/uuid"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store,error) {
	if db==nil { return nil,errors.New("prompt store requires database") }
	return &Store{db:db},nil
}

func (s *Store) Create(ctx context.Context,userID,projectID uuid.UUID,req Request)(Prompt,error){
	if err:=req.Validate(); err!=nil{return Prompt{},err}
	if valueOrEmpty(req.FinalText) == "" {
		if built := BuildFinalText(req.Components); built != "" {
			req.FinalText = &built
		}
	}
	aiRaw,_:=json.Marshal(req.AISuggestions); compRaw,_:=json.Marshal(req.Components)
	tx,err:=s.db.BeginTx(ctx,nil); if err!=nil{return Prompt{},err}
	defer tx.Rollback()
	var projectExists bool
	if err=tx.QueryRowContext(ctx,"SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')",projectID,userID).Scan(&projectExists); err!=nil{return Prompt{},err}
	if !projectExists{return Prompt{},ErrPromptNotFound}
	if _,err=tx.ExecContext(ctx,"SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))",projectID.String());err!=nil{return Prompt{},fmt.Errorf("lock prompt versions: %w",err)}
	version:=0
	if err=tx.QueryRowContext(ctx,"SELECT COALESCE(MAX(version),0)+1 FROM prompts WHERE project_id=$1",projectID).Scan(&version);err!=nil{return Prompt{},err}
	if req.ParentPromptID!=nil {
		var ok bool
		err=tx.QueryRowContext(ctx,"SELECT EXISTS (SELECT 1 FROM prompts WHERE id=$1 AND project_id=$2)",*req.ParentPromptID,projectID).Scan(&ok)
		if err!=nil{return Prompt{},err}; if !ok{return Prompt{},ErrPromptNotFound}
	}
	var p Prompt
	var aiBytes,compBytes []byte
	err=tx.QueryRowContext(ctx,`
		INSERT INTO prompts(project_id,iteration_id,parent_prompt_id,version,original_text,ai_suggestions,final_text,components,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id,project_id,iteration_id,parent_prompt_id,version,original_text,ai_suggestions,final_text,components,created_by,created_at
	`,projectID,req.IterationID,req.ParentPromptID,version,strings.TrimSpace(req.OriginalText),aiRaw,req.FinalText,compRaw,req.CreatedBy).Scan(
		&p.ID,&p.ProjectID,&p.IterationID,&p.ParentPromptID,&p.Version,&p.OriginalText,&aiBytes,&p.FinalText,&compBytes,&p.CreatedBy,&p.CreatedAt)
	if err!=nil{return Prompt{},fmt.Errorf("create prompt: %w",err)}
	_ = json.Unmarshal(aiBytes,&p.AISuggestions); _ = json.Unmarshal(compBytes,&p.Components)
	if err=tx.Commit();err!=nil{return Prompt{},err}
	return p,nil
}

func (s *Store) List(ctx context.Context,userID,projectID uuid.UUID)([]Prompt,error){
	rows,err:=s.db.QueryContext(ctx,`
		SELECT pr.id,pr.project_id,pr.iteration_id,pr.parent_prompt_id,pr.version,pr.original_text,pr.ai_suggestions,pr.final_text,pr.components,pr.created_by,pr.created_at
		FROM prompts pr JOIN projects p ON p.id=pr.project_id
		WHERE pr.project_id=$1 AND p.user_id=$2 AND p.status <> 'deleted'
		ORDER BY pr.version ASC
	`,projectID,userID); if err!=nil{return nil,err}; defer rows.Close()
	var out []Prompt
	for rows.Next(){var p Prompt;var a,c []byte;if err:=rows.Scan(&p.ID,&p.ProjectID,&p.IterationID,&p.ParentPromptID,&p.Version,&p.OriginalText,&a,&p.FinalText,&c,&p.CreatedBy,&p.CreatedAt);err!=nil{return nil,err};_ = json.Unmarshal(a,&p.AISuggestions);_ = json.Unmarshal(c,&p.Components);out=append(out,p)}
	return out,rows.Err()
}

func (s *Store) Get(ctx context.Context,userID,promptID uuid.UUID)(Prompt,error){
	var p Prompt;var a,c []byte
	err:=s.db.QueryRowContext(ctx,`
		SELECT pr.id,pr.project_id,pr.iteration_id,pr.parent_prompt_id,pr.version,pr.original_text,pr.ai_suggestions,pr.final_text,pr.components,pr.created_by,pr.created_at
		FROM prompts pr JOIN projects p ON p.id=pr.project_id
		WHERE pr.id=$1 AND p.user_id=$2 AND p.status <> 'deleted'
	`,promptID,userID).Scan(&p.ID,&p.ProjectID,&p.IterationID,&p.ParentPromptID,&p.Version,&p.OriginalText,&a,&p.FinalText,&c,&p.CreatedBy,&p.CreatedAt)
	if errors.Is(err,sql.ErrNoRows){return Prompt{},ErrPromptNotFound};if err!=nil{return Prompt{},err}
	_ = json.Unmarshal(a,&p.AISuggestions);_ = json.Unmarshal(c,&p.Components);return p,nil
}

func valueOrEmpty(value *string) string { if value == nil { return "" }; return *value }
