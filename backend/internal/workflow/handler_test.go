package workflow

import "testing"

func TestModePolicies(t *testing.T) {
 tests:=[]struct{mode Mode; variants int; priority string}{
  {ModeExplore,8,"high"},{ModeDevelop,4,"normal"},{ModeFinalize,1,"low"},
 }
 for _,tt:=range tests { p:=policy(tt.mode); if p.MaxVariants!=tt.variants||p.GenerationPriority!=tt.priority { t.Fatalf("%s: %#v",tt.mode,p) } }
}
func TestValidMode(t *testing.T) {
 for _,m:=range []Mode{ModeExplore,ModeDevelop,ModeFinalize}{if !validMode(m){t.Fatal(m)}}
 if validMode(Mode("bad")){t.Fatal("invalid mode accepted")}
}

func TestValidateOverrides(t *testing.T) {
    if err := validateOverrides([]string{}, nil, ""); err != nil { t.Fatalf("unexpected error: %v", err) }
    if err := validateOverrides([]string{"similarity"}, []string{"similarity"}, "documented skip"); err != nil { t.Fatalf("unexpected error: %v", err) }
    if err := validateOverrides([]string{"similarity"}, nil, "documented skip"); err == nil { t.Fatal("expected missing override error") }
    if err := validateOverrides([]string{"similarity"}, []string{"similarity"}, ""); err == nil { t.Fatal("expected missing reason error") }
    if err := validateOverrides([]string{"similarity"}, []string{"references"}, "documented"); err == nil { t.Fatal("expected non-warning override error") }
    if err := validateOverrides([]string{"similarity"}, []string{"similarity", "similarity"}, "documented"); err == nil { t.Fatal("expected duplicate override error") }
}