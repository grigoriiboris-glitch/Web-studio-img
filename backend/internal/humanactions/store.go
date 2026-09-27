package humanactions

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "fmt"
 "github.com/google/uuid"
)

type Store struct{ db *sql.DB }
func NewStore(db *sql.DB)(*Store,error){if db==nil{return nil,errors.New("human actions store requires database")};return &Store{db:db},nil}
func marshal(v map[string]any)([]byte,error){if v==nil{return []byte("{}"),nil};return json.Marshal(v)}
func(s *Store)Create(ctx context.Context,userID,projectID uuid.UUID,req Request)(Action,error){
 if err:=req.Validate(projectID,userID);err!=nil{return Action{},err}
 p,err:=marshal(req.Payload);if err!=nil{return Action{},err};o,err:=marshal(req.OldState);if err!=nil{return Action{},err};n,err:=marshal(req.NewState);if err!=nil{return Action{},err};ai,err:=marshal(req.AIInfluence);if err!=nil{return Action{},err}
 tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return Action{},err};defer func() { _ = tx.Rollback() }()
 var ok bool;if err=tx.QueryRowContext(ctx,"SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')",projectID,userID).Scan(&ok);err!=nil{return Action{},err};if !ok{return Action{},fmt.Errorf("%w: project not found",ErrInvalidAction)}
 if _,err=tx.ExecContext(ctx,"SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))","human-actions:"+projectID.String());err!=nil{return Action{},err}
 var version int64;if err=tx.QueryRowContext(ctx,"SELECT COALESCE(MAX(version),0)+1 FROM human_actions WHERE project_id=$1",projectID).Scan(&version);err!=nil{return Action{},err}
 var a Action;var pb,ob,nb,ab []byte
 err=tx.QueryRowContext(ctx,"INSERT INTO human_actions(project_id,iteration_id,user_id,version,action_type,payload,old_state,new_state,ai_influence) VALUES($1,$2,$3,$4,$5,NULLIF($6,'{}')::jsonb,NULLIF($7,'{}')::jsonb,NULLIF($8,'{}')::jsonb,NULLIF($9,'{}')::jsonb) RETURNING id,project_id,iteration_id,user_id,version,action_type,payload,old_state,new_state,ai_influence,created_at",projectID,req.IterationID,userID,version,req.ActionType,p,string(o),string(n),string(ai)).Scan(&a.ID,&a.ProjectID,&a.IterationID,&a.UserID,&a.Version,&a.ActionType,&pb,&ob,&nb,&ab,&a.CreatedAt)
 if err!=nil{return Action{},fmt.Errorf("create human action: %w",err)};_=json.Unmarshal(pb,&a.Payload);if len(ob)>0{_=json.Unmarshal(ob,&a.OldState)};if len(nb)>0{_=json.Unmarshal(nb,&a.NewState)};if len(ab)>0{_=json.Unmarshal(ab,&a.AIInfluence)};if err=tx.Commit();err!=nil{return Action{},err};return a,nil
}
func(s *Store)List(ctx context.Context,userID,projectID uuid.UUID)([]Action,error){rows,err:=s.db.QueryContext(ctx,"SELECT h.id,h.project_id,h.iteration_id,h.user_id,h.version,h.action_type,h.payload,h.old_state,h.new_state,h.ai_influence,h.created_at FROM human_actions h JOIN projects p ON p.id=h.project_id WHERE h.project_id=$1 AND h.user_id=$2 AND p.user_id=$2 AND p.status <> 'deleted' ORDER BY h.version ASC",projectID,userID);if err!=nil{return nil,err};defer rows.Close();var out []Action;for rows.Next(){var a Action;var p,o,n,ai []byte;if err:=rows.Scan(&a.ID,&a.ProjectID,&a.IterationID,&a.UserID,&a.Version,&a.ActionType,&p,&o,&n,&ai,&a.CreatedAt);err!=nil{return nil,err};_=json.Unmarshal(p,&a.Payload);if len(o)>0{_=json.Unmarshal(o,&a.OldState)};if len(n)>0{_=json.Unmarshal(n,&a.NewState)};if len(ai)>0{_=json.Unmarshal(ai,&a.AIInfluence)};out=append(out,a)};return out,rows.Err()}
