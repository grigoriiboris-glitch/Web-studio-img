package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func TestHashEventUsesCanonicalPayloadPlusParentHash(t *testing.T) {
	parent := "parent-hash"
	event := Event{EntityType:"generation",EntityID:uuid.MustParse("11111111-1111-1111-1111-111111111111"),Action:"generation.completed",Payload:map[string]any{"b":2,"a":1},ParentHash:parent}
	got,err:=HashEvent(event);if err!=nil{t.Fatalf("HashEvent: %v",err)}
	sum:=sha256.Sum256([]byte(`{"a":1,"b":2}`+parent));want:=hex.EncodeToString(sum[:]);if got!=want{t.Fatalf("got %q, want %q",got,want)}
}
func TestCanonicalPayloadDeterministic(t *testing.T){a,err:=CanonicalPayload(map[string]any{"z":1,"a":2});if err!=nil{t.Fatal(err)};b,err:=CanonicalPayload(map[string]any{"a":2,"z":1});if err!=nil{t.Fatal(err)};if string(a)!=string(b){t.Fatalf("payload is not deterministic: %s != %s",a,b)}}

func TestVerifyChainAcceptsTenEvents(t *testing.T){
	items:=make([]Event,0,10);parent:="";for i:=int64(1);i<=10;i++{e:=Event{Sequence:i,ID:uuid.New(),EntityType:"generation",EntityID:uuid.New(),Action:"generation.progress",Payload:map[string]any{"n":i},ParentHash:parent};h,err:=HashEvent(e);if err!=nil{t.Fatal(err)};e.Hash=h;parent=h;items=append(items,e)}
	v:=VerifyChain(items,items[len(items)-1].Sequence,items[len(items)-1].Hash);if !v.Valid||v.EventsChecked!=10{t.Fatalf("unexpected verification: %#v",v)}
}
func TestVerifyChainDetectsTamperAndDeletion(t *testing.T){
	items:=make([]Event,0,10);parent:="";for i:=int64(1);i<=10;i++{e:=Event{Sequence:i,ID:uuid.New(),EntityType:"generation",EntityID:uuid.New(),Action:"generation.progress",Payload:map[string]any{"n":i},ParentHash:parent};h,_:=HashEvent(e);e.Hash=h;parent=h;items=append(items,e)}
	mutated:=append([]Event(nil),items...);mutated[4].Payload=map[string]any{"n":999};v:=VerifyChain(mutated,items[9].Sequence,items[9].Hash);if v.Valid{t.Fatal("expected payload tamper to invalidate chain")}
	deleted:=append([]Event(nil),items[:9]...);v=VerifyChain(deleted,items[9].Sequence,items[9].Hash);if v.Valid{t.Fatal("expected deleted last event to invalidate head")}
	middle:=append([]Event{},items[:4]...);middle=append(middle,items[5:]...);v=VerifyChain(middle,items[9].Sequence,items[9].Hash);if v.Valid{t.Fatal("expected deleted middle event to invalidate chain")}
}
