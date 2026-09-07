package smokeapp

import (
	"context"
	"fmt"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
)

func init() {
	command.Register("default", runDefaultProfile)
}

func runDefaultProfile(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return fmt.Errorf("usage: smoke default <create|apply|show> [environment]")
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke default show")
		}
		for _, spec := range environment.DefaultToolSpecs {
			fmt.Println(spec)
		}
		return nil
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke default create <environment>")
		}
		env, err := environment.Create(ctx, args[1])
		if err != nil {
			return err
		}
		if err := environment.AddDefaultTools(ctx, env.Name); err != nil {
			return err
		}
		fmt.Println(env.WorkFile)
		return nil
	case "apply":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke default apply <environment>")
		}
		return environment.AddDefaultTools(ctx, args[1])
	default:
		return fmt.Errorf("unknown default operation %q", args[0])
	}
}
