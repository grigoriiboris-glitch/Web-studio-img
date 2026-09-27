package provenance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID uuid.UUID
	UserID uuid.UUID
	ProjectID uuid.UUID
	IterationID *uuid.UUID
	EntityType string
	EntityID uuid.UUID
	Action string
	Payload map[string]any
	ParentHash string
	Hash string
	CreatedAt time.Time
}

type Verification struct {
	Verified bool `json:"verified"`
	EventCount int `json:"event_count"`
	FirstInvalidEventID *uuid.UUID `json:"first_invalid_event_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store,error) {
	if db==nil { return nil,errors.New("provenance store requires database") }
	return &Store{db:db},nil
}

type canonicalEvent struct {
	EntityType string `json:"entity_type"`
	EntityID uuid.UUID `json:"entity_id"`
	Action string `json:"action"`
	Payload map[string]any `json:"payload"`
	ParentHash string `json:"parent_hash,omitempty"`
	CreatedAt string `json:"created_at"`
}

func CanonicalBytes(event Event)([]byte,error){
	payload:=event.Payload
	if payload==nil{payload=map[string]any{}}
	return json.Marshal(canonicalEvent{EntityType:event.EntityType,EntityID:event.EntityID,Action:event.Action,Payload:payload,ParentHash:event.ParentHash,CreatedAt:event.CreatedAt.UTC().Format(time.RFC3339Nano)})
}
func HashEvent(event Event)(string,error){
	raw,err:=CanonicalBytes(event);if err!=nil{return "",err};sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:]),nil
}

func (s *Store) Append(ctx context.Context,event Event)(Event,error){
	if event.UserID==uuid.Nil||event.ProjectID==uuid.Nil||event.EntityID==uuid.Nil||event.Action==""{return Event{},errors.New("invalid provenance event")}
	if event.CreatedAt.IsZero(){event.CreatedAt=time.Now().UTC()}
	tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return Event{},err};defer tx.Rollback()
	var owner uuid.UUID
	if err:=tx.QueryRowContext(ctx,"SELECT user_id FROM projects WHERE id=$1 AND status <> 'deleted' FOR UPDATE",event.ProjectID).Scan(&owner);err!=nil{return Event{},fmt.Errorf("load provenance project: %w",err)}
	if owner!=event.UserID{return Event{},errors.New("project not owned")}
	_ = tx.QueryRowContext(ctx,"SELECT hash FROM provenance_events WHERE project_id=$1 ORDER BY sequence DESC LIMIT 1",event.ProjectID).Scan(&event.ParentHash)
	event.ID=uuid.New();event.Hash,err=HashEvent(event);if err!=nil{return Event{},fmt.Errorf("hash provenance event: %w",err)}
	rawPayload,err:=json.Marshal(event.Payload);if err!=nil{return Event{},fmt.Errorf("marshal provenance payload: %w",err)}
	_,err=tx.ExecContext(ctx,`INSERT INTO provenance_events(id,user_id,project_id,iteration_id,entity_type,entity_id,action,payload,parent_hash,hash,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,event.ID,event.UserID,event.ProjectID,event.IterationID,event.EntityType,event.EntityID,event.Action,rawPayload,nullableString(event.ParentHash),event.Hash,event.CreatedAt)
	if err!=nil{return Event{},fmt.Errorf("append provenance event: %w",err)}
	if err:=tx.Commit();err!=nil{return Event{},err};return event,nil
}

func (s *Store) Verify(ctx context.Context,userID,projectID uuid.UUID)(Verification,error){
	var owner uuid.UUID
	if err:=s.db.QueryRowContext(ctx,"SELECT user_id FROM projects WHERE id=$1 AND status <> 'deleted'",projectID).Scan(&owner);errors.Is(err,sql.ErrNoRows)||owner!=userID{return Verification{},fmt.Errorf("project not found")}else if err!=nil{return Verification{},err}
	rows,err:=s.db.QueryContext(ctx,`SELECT id,entity_type,entity_id,action,payload,parent_hash,hash,created_at FROM provenance_events WHERE project_id=$1 ORDER BY sequence ASC`,projectID);if err!=nil{return Verification{},err};defer rows.Close()
	result:=Verification{Verified:true};prev:=""
	for rows.Next(){var e Event;var raw []byte;var parent *string;if err:=rows.Scan(&e.ID,&e.EntityType,&e.EntityID,&e.Action,&raw,&parent,&e.Hash,&e.CreatedAt);err!=nil{return Verification{},err};if parent!=nil{e.ParentHash=*parent};if len(raw)>0{if err:=json.Unmarshal(raw,&e.Payload);err!=nil{return Verification{},err}};result.EventCount++;if e.ParentHash!=prev{result.Verified=false;result.FirstInvalidEventID=&e.ID;result.Reason="parent hash mismatch";break};expected,err:=HashEvent(e);if err!=nil{return Verification{},err};if e.Hash!=expected{result.Verified=false;result.FirstInvalidEventID=&e.ID;result.Reason="event hash mismatch";break};prev=e.Hash}
	if err:=rows.Err();err!=nil{return Verification{},err};return result,nil
}
func nullableString(value string)*string{if value==""{return nil};return &value}
