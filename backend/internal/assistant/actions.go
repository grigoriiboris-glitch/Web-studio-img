package assistant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Action struct {
	ID uuid.UUID \`json:"id"\`
	ProjectID uuid.UUID \`json:"project_id"\`
	UserID uuid.UUID \`json:"user_id"\`
	Tool string \`json:"tool"\`
	Kind string \`json:"kind"\`
	Input map[string]any \`json:"input"\`
	Output map[string]any \`json:"output"\`
	Explanation string \`json:"explanation"\`
	Confidence float64 \`json:"confidence"\`
	Uncertainty string \`json:"uncertainty"\`
	Decision *string \`json:"decision,omitempty"\`
	IdempotencyKey *string \`json:"-"\`
	DecisionIdempotencyKey *string \`json:"-"\`
	CreatedAt time.Time \`json:"created_at"\`
	DecidedAt *time.Time \`json:"decided_at,omitempty"\`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB)(*Store,error){if db==nil{return nil,errors.New("assistant action store requires database")};return &Store{db:db},nil}
func marshal(v map[string]any)([]byte,error){if v==nil{return []byte("{}"),nil};return json.Marshal(v)}

func (s *Store) Create(ctx context.Context,userID,projectID uuid.UUID,tool,kind string,input,output map[string]any,explanation string,confidence float64,uncertainty,key string)(Action,error){
	in,e:=marshal(input);if e!=nil{return Action{},e};out,e:=marshal(output);if e!=nil{return Action{},e};var a Action;var ib,ob []byte
	e=s.db.QueryRowContext(ctx,\`
		INSERT INTO assistant_actions(project_id,user_id,tool,kind,input,output,explanation,confidence,uncertainty,idempotency_key)
		SELECT p.id,$2,$3,$4,$5::jsonb,$6::jsonb,$7,$8,$9,NULLIF($10,'') FROM projects p
		WHERE p.id=$1 AND p.user_id=$2 AND p.status <> 'deleted'
		ON CONFLICT (user_id,idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING id,project_id,user_id,tool,kind,input,output,explanation,confidence,uncertainty,decision,idempotency_key,decision_idempotency_key,created_at,decided_at
	\`,projectID,userID,tool,kind,string(in),string(out),explanation,confidence,uncertainty,key).Scan(&a.ID,&a.ProjectID,&a.UserID,&a.Tool,&a.Kind,&ib,&ob,&a.Explanation,&a.Confidence,&a.Uncertainty,&a.Decision,&a.IdempotencyKey,&a.DecisionIdempotencyKey,&a.CreatedAt,&a.DecidedAt)
	if errors.Is(e,sql.ErrNoRows)&&key!=""{return s.GetByIdempotencyKey(ctx,userID,projectID,key)}
	if e!=nil{return Action{},e};_=json.Unmarshal(ib,&a.Input);_=json.Unmarshal(ob,&a.Output);return a,nil
}

func (s *Store) GetByIdempotencyKey(ctx context.Context,userID,projectID uuid.UUID,key string)(Action,error){
	var a Action;var ib,ob []byte
	e:=s.db.QueryRowContext(ctx,\`SELECT id,project_id,user_id,tool,kind,input,output,explanation,confidence,uncertainty,decision,idempotency_key,decision_idempotency_key,created_at,decided_at FROM assistant_actions WHERE user_id=$1 AND project_id=$2 AND idempotency_key=$3\`,userID,projectID,key).Scan(&a.ID,&a.ProjectID,&a.UserID,&a.Tool,&a.Kind,&ib,&ob,&a.Explanation,&a.Confidence,&a.Uncertainty,&a.Decision,&a.IdempotencyKey,&a.DecisionIdempotencyKey,&a.CreatedAt,&a.DecidedAt)
	if e!=nil{return Action{},e};_=json.Unmarshal(ib,&a.Input);_=json.Unmarshal(ob,&a.Output);return a,nil
}

func (s *Store) GetOwned(ctx context.Context,userID,projectID,actionID uuid.UUID)(Action,error){
	var a Action;var ib,ob []byte
	e:=s.db.QueryRowContext(ctx,\`SELECT a.id,a.project_id,a.user_id,a.tool,a.kind,a.input,a.output,a.explanation,a.confidence,a.uncertainty,a.decision,a.idempotency_key,a.decision_idempotency_key,a.created_at,a.decided_at FROM assistant_actions a JOIN projects p ON p.id=a.project_id WHERE a.id=$1 AND a.project_id=$2 AND a.user_id=$3 AND p.user_id=$3 AND p.status <> 'deleted'\`,actionID,projectID,userID).Scan(&a.ID,&a.ProjectID,&a.UserID,&a.Tool,&a.Kind,&ib,&ob,&a.Explanation,&a.Confidence,&a.Uncertainty,&a.Decision,&a.IdempotencyKey,&a.DecisionIdempotencyKey,&a.CreatedAt,&a.DecidedAt)
	if e!=nil{return Action{},e};_=json.Unmarshal(ib,&a.Input);_=json.Unmarshal(ob,&a.Output);return a,nil
}

func (s *Store) GetByDecisionIdempotencyKey(ctx context.Context,userID,projectID uuid.UUID,key string)(Action,error){
	var a Action;var ib,ob []byte
	e:=s.db.QueryRowContext(ctx,\`SELECT a.id,a.project_id,a.user_id,a.tool,a.kind,a.input,a.output,a.explanation,a.confidence,a.uncertainty,a.decision,a.idempotency_key,a.decision_idempotency_key,a.created_at,a.decided_at FROM assistant_actions a JOIN projects p ON p.id=a.project_id WHERE a.user_id=$1 AND a.project_id=$2 AND a.decision_idempotency_key=$3\`,userID,projectID,key).Scan(&a.ID,&a.ProjectID,&a.UserID,&a.Tool,&a.Kind,&ib,&ob,&a.Explanation,&a.Confidence,&a.Uncertainty,&a.Decision,&a.IdempotencyKey,&a.DecisionIdempotencyKey,&a.CreatedAt,&a.DecidedAt)
	if e!=nil{return Action{},e};_=json.Unmarshal(ib,&a.Input);_=json.Unmarshal(ob,&a.Output);return a,nil
}

func (s *Store) Decide(ctx context.Context,userID,projectID,actionID uuid.UUID,decision,key string,output map[string]any)(Action,error){
	if decision!="apply"&&decision!="edit"&&decision!="ignore"{return Action{},errors.New("invalid decision")};raw,e:=marshal(output);if e!=nil{return Action{},e}
	var a Action;var ib,ob []byte;now:=time.Now().UTC()
	e=s.db.QueryRowContext(ctx,\`
		UPDATE assistant_actions a SET decision=$1,output=$2::jsonb,decision_idempotency_key=NULLIF($3,''),decided_at=$4
		FROM projects p
		WHERE a.id=$5 AND a.project_id=$6 AND a.user_id=$7 AND a.kind='recommendation' AND a.decision IS NULL
		  AND p.id=a.project_id AND p.user_id=$7 AND p.status <> 'deleted'
		RETURNING a.id,a.project_id,a.user_id,a.tool,a.kind,a.input,a.output,a.explanation,a.confidence,a.uncertainty,a.decision,a.idempotency_key,a.decision_idempotency_key,a.created_at,a.decided_at
	\`,decision,string(raw),key,now,actionID,projectID,userID).Scan(&a.ID,&a.ProjectID,&a.UserID,&a.Tool,&a.Kind,&ib,&ob,&a.Explanation,&a.Confidence,&a.Uncertainty,&a.Decision,&a.IdempotencyKey,&a.DecisionIdempotencyKey,&a.CreatedAt,&a.DecidedAt)
	if errors.Is(e,sql.ErrNoRows)&&key!=""{return s.GetByDecisionIdempotencyKey(ctx,userID,projectID,key)}
	if e!=nil{return Action{},e};_=json.Unmarshal(ib,&a.Input);_=json.Unmarshal(ob,&a.Output);return a,nil
}

func (s *Store) List(ctx context.Context,userID,projectID uuid.UUID)([]Action,error){
	rows,e:=s.db.QueryContext(ctx,\`SELECT a.id,a.project_id,a.user_id,a.tool,a.kind,a.input,a.output,a.explanation,a.confidence,a.uncertainty,a.decision,a.idempotency_key,a.decision_idempotency_key,a.created_at,a.decided_at FROM assistant_actions a JOIN projects p ON p.id=a.project_id WHERE a.project_id=$1 AND a.user_id=$2 AND p.user_id=$2 AND p.status <> 'deleted' ORDER BY a.created_at ASC,a.id ASC\`,projectID,userID)
	if e!=nil{return nil,e};defer rows.Close();var out []Action
	for rows.Next(){var a Action;var ib,ob []byte;if e:=rows.Scan(&a.ID,&a.ProjectID,&a.UserID,&a.Tool,&a.Kind,&ib,&ob,&a.Explanation,&a.Confidence,&a.Uncertainty,&a.Decision,&a.IdempotencyKey,&a.DecisionIdempotencyKey,&a.CreatedAt,&a.DecidedAt);e!=nil{return nil,e};_=json.Unmarshal(ib,&a.Input);_=json.Unmarshal(ob,&a.Output);out=append(out,a)}
	return out,rows.Err()
}
