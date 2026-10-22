package versioning

import (
 "errors"
 "testing"
)

func TestParse(t *testing.T) {
 good:=[]string{"2026.10.21","2026.10.21.0","2026.10.21.1","2024.02.29","2026.12.31.18446744073709551615"}
 for _,s:=range good { v,err:=Parse(s);if err!=nil || v.String()!=s {t.Errorf("%q => %v, %v",s,v,err)} }
 bad:=[]string{"","v2026.10.21","2026.1.21","2026.10.1","2026.02.29","2026.13.01","2026.00.01","2026.04.31","0000.01.01","2026.10.21.01","2026.10.21.18446744073709551616"}
 for _,s:=range bad {if _,err:=Parse(s);err==nil {t.Errorf("accepted %q",s)}}
}
func TestReference(t *testing.T) {
 for _,s:=range []string{"localhost/fatline/nginx:2026.10.21","service:qualified-2026.10.21.1","artifact:candidate-2026.10.21.2"} {
  r,err:=ParseReference(s);if err!=nil || r.String()!=s {t.Errorf("%q => %v %v",s,r,err)}
 }
 for _,s:=range []string{"service:qualified","service:latest","service:qualified-2026.02.29","service:candidate-2026.10.21.01"} {if _,err:=ParseReference(s);err==nil {t.Errorf("accepted %q",s)}}
}
func TestBinding(t *testing.T) {
 ref,_:=ParseReference("service:2026.10.21")
 old:=ImmutableBinding{Reference:ref,ArtifactDigest:"sha256:abc"}
 if err:=CheckBinding(nil,old);err!=nil {t.Fatal(err)}
 if err:=CheckBinding(&old,old);err!=nil {t.Fatal(err)}
 next:=old;next.ArtifactDigest="sha256:def"
 if err:=CheckBinding(&old,next);!errors.Is(err,ErrBindingConflict) {t.Fatalf("expected conflict: %v",err)}
}
