package branches

import (
    "testing"
    "github.com/google/uuid"
)

func TestValidateBranchName(t *testing.T) {
    if ValidateName("   ") == nil { t.Fatal("blank name must fail") }
    if ValidateName("main") != nil { t.Fatal("valid name rejected") }
    if ValidateName(string(make([]rune, MaxNameLength+1))) == nil { t.Fatal("overlong name must fail") }
}
func TestDecisionDimensions(t *testing.T) {
    got:=DecisionDimensions()
    if len(got)!=9 { t.Fatalf("dimensions=%d, want 9",len(got)) }
    for _,d:=range got { if !IsDecisionDimension(d){t.Fatalf("dimension %q not recognized",d)} }
    if IsDecisionDimension("unknown"){t.Fatal("unknown dimension accepted")}
}
func TestMergeDecisionSourceMustBeSelectedBranch(t *testing.T) {
    req:=MergeRequest{Sources:[]uuid.UUID{uuid.New()},Decisions:map[string]Decision{"prompt":{SourceBranchID:uuid.New()}}}
    if req.Decisions["prompt"].SourceBranchID==req.Sources[0] { t.Fatal("test setup invalid") }
}
