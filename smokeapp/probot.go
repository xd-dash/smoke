package smokeapp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
)

const probotEnvironment = "probot"
const probotToolSpec = "github.com/xd-dash/probot-runtime/cmd/probot-runtime-api@36b60cb0415eb5a57b9cc60f4a54a4fe1252d0f1"

func init() { command.Register("probot", runProbot) }

// This optional tool environment composes Probot's Go HTTP client but does not
// embed the application runtime, its private credentials, or a server.
func runProbot(args []string) error {
	ctx := context.Background()
	if len(args) == 0 { return fmt.Errorf("usage: smoke probot <bootstrap|tool> [args...]") }
	switch args[0] {
	case "bootstrap":
		name := probotEnvironment
		if len(args) > 2 { return fmt.Errorf("usage: smoke probot bootstrap [environment]") }
		if len(args) == 2 { name = args[1] }
		if _,err := environment.Require(name);err!=nil {
			if _,err = environment.Create(ctx,name);err!=nil { return err }
		}
		if err:=environment.AddTool(ctx,name,probotToolSpec);err!=nil { return err }
		fmt.Printf("SMOKE_PROBOT_TOOL=passed environment=%s revision=%s\n",name,strings.Split(probotToolSpec,"@")[1])
		return nil
	case "tool":
		name,rest:=probotEnvironment,args[1:]
		if len(rest)>=2 && rest[0]=="--env" { name,rest=rest[1],rest[2:] }
		if len(rest)==0 { return fmt.Errorf("usage: smoke probot tool [--env NAME] [--endpoint URL] [--token-file FILE] <ready|delivery-run|reconcile>") }
		env,err:=environment.Require(name)
		if err!=nil { return fmt.Errorf("Probot tool environment %q is not bootstrapped: %w",name,err) }
		workspace,err:=environment.Snapshot(ctx,env)
		if err!=nil { return err }
		goBin,err:=exec.LookPath("go")
		if err!=nil { return fmt.Errorf("Probot Go tool requires Go: %w",err) }
		return runCommand(workspace.Command(ctx,workspace.ToolsDir,goBin,append([]string{"tool","probot-runtime-api"},rest...)...))
	default:
		return fmt.Errorf("unknown probot operation %q",args[0])
	}
}
