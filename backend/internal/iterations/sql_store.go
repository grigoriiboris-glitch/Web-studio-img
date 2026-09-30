package iterations

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/google/uuid"
)
type SQLStore struct{db *sql.DB}
func NewSQLStore(db *sql.DB)(*SQLStore,error){if db==nil{return nil,errors.New("iteration store requires database")};return &SQLStore{db:db},nil}
func(s *SQLStore)Create(ctx context.Context,userID,projectID uuid.UUID,parentID *uuid.UUID,t Type,title,description *string)(Iteration,error){
    if err:=ValidateType(t);err!=nil{return Iteration{},err};if err:=ValidateTitle(title);err!=nil{return Iteration{},err};if err:=ValidateDescription(description);err!=nil{return Iteration{},err};if err:=ValidateManualEditDescription(t,description);err!=nil{return Iteration{},err}
    branchID,err:=s.resolveBranch(ctx,userID,projectID,parentID);if err!=nil{return Iteration{},err}
    return s.insert(ctx,projectID,branchID,parentID,t,title,description,map[string]any{},nil)
}
func(s *SQLStore)CreateWithDecisions(ctx context.Context,userID,projectID,branchID uuid.UUID,parentID *uuid.UUID,t Type,title,description *string,decisions map[string]any,sources []uuid.UUID)(Iteration,error){
    if err:=ValidateType(t);err!=nil{return Iteration{},err};if err:=ValidateTitle(title);err!=nil{return Iteration{},err};if err:=ValidateDescription(description);err!=nil{return Iteration{},err};if err:=ValidateManualEditDescription(t,description);err!=nil{return Iteration{},err};if err:=ValidateDecisions(decisions);err!=nil{return Iteration{},err}
    var ok bool;if err:=s.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM projects p JOIN branches b ON b.project_id=p.id WHERE p.id=$1 AND p.user_id=$2 AND p.status<>'deleted' AND b.id=$3)`,projectID,userID,branchID).Scan(&ok);err!=nil{return Iteration{},err};if !ok{return Iteration{},ErrIterationNotFound}
    return s.insert(ctx,projectID,branchID,parentID,t,title,description,decisions,sources)
}
func(s *SQLStore)resolveBranch(ctx context.Context,userID,projectID uuid.UUID,parentID *uuid.UUID)(uuid.UUID,error){
    var id uuid.UUID
    if parentID!=nil{err:=s.db.QueryRowContext(ctx,`SELECT i.branch_id FROM iterations i JOIN projects p ON p.id=i.project_id WHERE i.id=$1 AND i.project_id=$2 AND p.user_id=$3 AND p.status<>'deleted'`,*parentID,projectID,userID).Scan(&id);if errors.Is(err,sql.ErrNoRows){return uuid.Nil,ErrIterationNotFound};if err!=nil{return uuid.Nil,err};return id,nil}
    err:=s.db.QueryRowContext(ctx,`SELECT b.id FROM branches b JOIN projects p ON p.id=b.project_id WHERE b.project_id=$1 AND p.user_id=$2 AND p.status<>'deleted' AND b.name='main' AND b.status='active'`,projectID,userID).Scan(&id);if errors.Is(err,sql.ErrNoRows){return uuid.Nil,ErrIterationNotFound};return id,err
}
func(s *SQLStore)Get(ctx context.Context,userID,id uuid.UUID)(Iteration,error){return s.getOwned(ctx,userID,id)}
func(s *SQLStore)List(ctx context.Context,userID,projectID uuid.UUID)([]Iteration,error){
    rows,err:=s.db.QueryContext(ctx,`SELECT i.id,i.project_id,i.branch_id,i.parent_iteration_id,i.type,i.title,i.description,i.decisions,i.merge_sources,i.created_at FROM iterations i JOIN projects p ON p.id=i.project_id WHERE i.project_id=$1 AND p.user_id=$2 AND p.status<>'deleted' ORDER BY i.created_at ASC,i.id ASC`,projectID,userID);if err!=nil{return nil,fmt.Errorf("list iterations: %w",err)};defer func() { _ = rows.Close() }();out:=[]Iteration{};for rows.Next(){v,e:=scanIteration(rows);if e!=nil{return nil,e};out=append(out,v)};return out,rows.Err()
}
func(s *SQLStore)Restore(ctx context.Context,userID,id uuid.UUID)(Iteration,error){src,e:=s.getOwned(ctx,userID,id);if e!=nil{return Iteration{},e};return s.insert(ctx,src.ProjectID,src.BranchID,&src.ID,src.Type,src.Title,src.Description,src.Decisions,src.MergeSources)}
func(s *SQLStore)getOwned(ctx context.Context,userID,id uuid.UUID)(Iteration,error){
    row:=s.db.QueryRowContext(ctx,`SELECT i.id,i.project_id,i.branch_id,i.parent_iteration_id,i.type,i.title,i.description,i.decisions,i.merge_sources,i.created_at FROM iterations i JOIN projects p ON p.id=i.project_id WHERE i.id=$1 AND p.user_id=$2 AND p.status<>'deleted'`,id,userID);v,e:=scanIteration(row);if errors.Is(e,sql.ErrNoRows){return Iteration{},ErrIterationNotFound};if e!=nil{return Iteration{},fmt.Errorf("get iteration: %w",e)};return v,nil
}
func(s *SQLStore)insert(ctx context.Context,projectID,branchID uuid.UUID,parentID *uuid.UUID,t Type,title,description *string,decisions map[string]any,sources []uuid.UUID)(Iteration,error){
    raw,_:=json.Marshal(decisions);src,_:=json.Marshal(sources);row:=s.db.QueryRowContext(ctx,`INSERT INTO iterations(project_id,branch_id,parent_iteration_id,type,title,description,decisions,merge_sources) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,project_id,branch_id,parent_iteration_id,type,title,description,decisions,merge_sources,created_at`,projectID,branchID,parentID,t,title,description,raw,src);v,e:=scanIteration(row);if e!=nil{return Iteration{},fmt.Errorf("create iteration: %w",e)};return v,nil
}
type scanner interface{Scan(...any)error}
func scanIteration(s scanner)(Iteration,error){var v Iteration;var decisions, sources []byte;if err:=s.Scan(&v.ID,&v.ProjectID,&v.BranchID,&v.ParentIterationID,&v.Type,&v.Title,&v.Description,&decisions,&sources,&v.CreatedAt);err!=nil{return Iteration{},err};v.Decisions=map[string]any{};v.MergeSources=[]uuid.UUID{};if len(decisions)>0{if err:=json.Unmarshal(decisions,&v.Decisions);err!=nil{return Iteration{},err}};if len(sources)>0{if err:=json.Unmarshal(sources,&v.MergeSources);err!=nil{return Iteration{},err}};return v,nil}
