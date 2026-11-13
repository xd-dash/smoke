package smokeapp

import "testing"

func TestProbotOptionalCommandIsRegistered(t *testing.T) {
	if err:=runProbot(nil);err==nil { t.Fatal("expected usage failure") }
	if err:=runProbot([]string{"tool"});err==nil { t.Fatal("expected missing tool invocation failure") }
	if err:=runProbot([]string{"bootstrap","one","extra"});err==nil { t.Fatal("expected invalid bootstrap arguments") }
	if err:=runProbot([]string{"unsupported"});err==nil { t.Fatal("accepted unknown operation") }
}
