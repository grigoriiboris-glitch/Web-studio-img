package exports

import "testing"

func TestSHA256Hex(t *testing.T){
 if got:=sha256Hex([]byte("web-studio-img")); got!="dd5b8f2e0d3b1d5e9bb4cfcf8c7d6f0a6d8e7c9a6b4f7f7d0e7b3b1a6c7b0f9e"{t.Fatalf("unexpected hash %s",got)}
}
func TestCountJSON(t *testing.T){
 if countJSON([]byte("[1,2,3]"))!=3{t.Fatal("count mismatch")}
 if countJSON([]byte("bad"))!=0{t.Fatal("invalid JSON must be zero")}
}
