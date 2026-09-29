package exports

import ("testing"
 "github.com/google/uuid"
)

func TestSHA256Hex(t *testing.T){
 if got:=sha256Hex([]byte("web-studio-img")); got!="4511f86c2180dd6fce0cc9d9776bf05e08786d51ada7f9aaa6969029d467d676"{t.Fatalf("unexpected hash %s",got)}
}
func TestMaskArtifactName(t *testing.T) {
 id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
 if got := maskArtifactName(id); got != "mask/00000000-0000-0000-0000-000000000001" { t.Fatalf("unexpected mask artifact name %s", got) }
}

func TestCountJSON(t *testing.T){
 if countJSON([]byte("[1,2,3]"))!=3{t.Fatal("count mismatch")}
 if countJSON([]byte("bad"))!=0{t.Fatal("invalid JSON must be zero")}
}
