package manualedits

import "testing"

func TestValidOperation(t *testing.T){
	for _,v:=range []Operation{OperationPaint,OperationErase,OperationMask,OperationComposite}{if !validOperation(v){t.Fatalf("operation %q should be valid",v)}}
	if validOperation(Operation("unknown")){t.Fatal("unknown operation should be rejected")}
}
func TestEffectiveDescription(t *testing.T){
	p:="remove object"
	if got:=effectiveDescription(OperationErase,&p);got!="erase: remove object"{t.Fatalf("got %q",got)}
	if got:=effectiveDescription(OperationMask,nil);got!="mask"{t.Fatalf("got %q",got)}
}
