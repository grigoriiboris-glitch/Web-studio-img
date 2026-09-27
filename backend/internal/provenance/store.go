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
 Sequence int64 `json:"-"`
 ID uuid.UUID `json:"id"`
 UserID uuid.UUID `json:"user_id"`
 ProjectID uuid.UUID `json:"project_id"`
 IterationID *uuid.UUID `json:"iteration_id,omitempty"`
 EntityType string `json:"entity_type"`
 EntityID uuid.UUID `json:"entity_id"`
 Action string `json:"action"`
 Payload map[string]any `json:"payload,omitempty"`
 ParentHash string `json:"parent_hash,omitempty"`
 Hash string `json:"hash"`
 CreatedAt time.Time `json:"created_at"`
}

type Verification struct { Valid bool `json:"valid"`; EventsChecked int `json:"events_checked"`; BrokenLinks []string `json:"broken_links"`; VerifiedAt time.Time `json:"verified_at"` }

type Store struct{ db *sql.DB }
func NewStore(db *sql.DB)(*Store,error){if db==nil{return nil,errors.New("provenance store requires database")};return &Store{db:db},nil}
func CanonicalPayload(payload map[string]any)([]byte,error){if payload==nil{payload=map[string]any{}};return json.Marshal(payload)}
func HashEvent(event Event)(string,error){payload,err:=CanonicalPayload(event.Payload);if err!=nil{return "",err};raw:=append(append([]byte{},payload...),[]byte(event.ParentHash)...);sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:]),nil}
func(s *Store)Append(ctx context.Context,event Event)(Event,error){
 if event.UserID==uuid.Nil||event.ProjectID==uuid.Nil||event.EntityID==uuid.Nil||event.Action==""{return Event{},errors.New("invalid provenance event")};if event.CreatedAt.IsZero(){event.CreatedAt=time.Now().UTC()}
 tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return Event{},err};defer func() { _ = tx.Rollback() }();var owner uuid.UUID
 if err:=tx.QueryRowContext(ctx,"SELECT user_id FROM projects WHERE id=$1 AND status <> 'deleted' FOR UPDATE",event.ProjectID).Scan(&owner);err!=nil{return Event{},fmt.Errorf("load provenance project: %w",err)};if owner!=event.UserID{return Event{},errors.New("project not owned")}
 _=tx.QueryRowContext(ctx,"SELECT hash FROM provenance_events WHERE project_id=$1 ORDER BY sequence DESC LIMIT 1",event.ProjectID).Scan(&event.ParentHash);event.ID=uuid.New();event.Hash,err=HashEvent(event);if err!=nil{return Event{},err};raw,err:=json.Marshal(event.Payload);if err!=nil{return Event{},err}
 err=tx.QueryRowContext(ctx,"INSERT INTO provenance_events(id,user_id,project_id,iteration_id,entity_type,entity_id,action,payload,parent_hash,hash,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING sequence",event.ID,event.UserID,event.ProjectID,event.IterationID,event.EntityType,event.EntityID,event.Action,raw,nullableString(event.ParentHash),event.Hash,event.CreatedAt).Scan(&event.Sequence);if err!=nil{return Event{},err}
 if _,err=tx.ExecContext(ctx,"UPDATE projects SET provenance_head_sequence=$2, provenance_head_hash=$3 WHERE id=$1",event.ProjectID,event.Sequence,event.Hash);err!=nil{return Event{},err}
 if err:=tx.Commit();err!=nil{return Event{},err};return event,nil
}
func(s *Store)List(ctx context.Context,userID,projectID uuid.UUID)([]Event,error){var owner uuid.UUID;if err:=s.db.QueryRowContext(ctx,"SELECT user_id FROM projects WHERE id=$1 AND status <> 'deleted'",projectID).Scan(&owner);errors.Is(err,sql.ErrNoRows)||owner!=userID{return nil,fmt.Errorf("project not found")}else if err!=nil{return nil,err};rows,err:=s.db.QueryContext(ctx,"SELECT sequence,id,user_id,project_id,iteration_id,entity_type,entity_id,action,payload,parent_hash,hash,created_at FROM provenance_events WHERE project_id=$1 ORDER BY sequence ASC",projectID);if err!=nil{return nil,err};defer func() { _ = rows.Close() }();var out []Event;for rows.Next(){var e Event;var raw []byte;var parent *string;if err:=rows.Scan(&e.Sequence,&e.ID,&e.UserID,&e.ProjectID,&e.IterationID,&e.EntityType,&e.EntityID,&e.Action,&raw,&parent,&e.Hash,&e.CreatedAt);err!=nil{return nil,err};if parent!=nil{e.ParentHash=*parent};if len(raw)>0{if err:=json.Unmarshal(raw,&e.Payload);err!=nil{return nil,err}};out=append(out,e)};return out,rows.Err()}
func VerifyChain(items []Event, headSequence int64, headHash string) Verification {
 result:=Verification{Valid:true,BrokenLinks:[]string{},VerifiedAt:time.Now().UTC()}
 prev:="";var prevSeq int64
 for _,e:=range items {
  result.EventsChecked++
  if e.ParentHash!=prev { result.Valid=false;result.BrokenLinks=append(result.BrokenLinks,e.ID.String()+": parent hash mismatch") }
  if prevSeq!=0 && e.Sequence!=prevSeq+1 { result.Valid=false;result.BrokenLinks=append(result.BrokenLinks,e.ID.String()+": sequence gap") }
  expected,err:=HashEvent(e);if err!=nil{result.Valid=false;result.BrokenLinks=append(result.BrokenLinks,e.ID.String()+": hash calculation failed");continue}
  if e.Hash!=expected { result.Valid=false;result.BrokenLinks=append(result.BrokenLinks,e.ID.String()+": event hash mismatch") }
  prev=e.Hash;prevSeq=e.Sequence
 }
 if len(items)==0 {
  if headSequence!=0 || headHash!="" { result.Valid=false;result.BrokenLinks=append(result.BrokenLinks,"provenance head points to missing event") }
 } else if items[len(items)-1].Sequence!=headSequence || items[len(items)-1].Hash!=headHash {
  result.Valid=false;result.BrokenLinks=append(result.BrokenLinks,"provenance head mismatch")
 }
 return result
}
func(s *Store)Verify(ctx context.Context,userID,projectID uuid.UUID)(Verification,error){
 items,err:=s.List(ctx,userID,projectID);if err!=nil{return Verification{},err}
 var headSequence int64;var headHash string
 if err:=s.db.QueryRowContext(ctx,"SELECT COALESCE(provenance_head_sequence,0),COALESCE(provenance_head_hash,'') FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted'",projectID,userID).Scan(&headSequence,&headHash);err!=nil{return Verification{},err}
 return VerifyChain(items,headSequence,headHash),nil
}
func nullableString(value string)*string{if value==""{return nil};return &value}
