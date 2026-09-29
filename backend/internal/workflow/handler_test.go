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
