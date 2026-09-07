package smokeapp

import (
	"context"
	"fmt"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/ghxd"
)

func init() {
	command.Register("ghxd", runGHXD)
}

func runGHXD(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return ghxdUsage()
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke ghxd show")
		}
		fmt.Printf("environment\t%s\n", ghxd.DefaultEnvironment)
		for _, spec := range ghxd.ToolSpecs {
			fmt.Printf("tool\t%s\n", spec)
		}
		return nil
	case "bootstrap":
		if len(args) > 2 {
			return fmt.Errorf("usage: smoke ghxd bootstrap [environment]")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		env, err := ghxd.Bootstrap(ctx, name)
		if err != nil {
			return err
		}
		fmt.Printf("ghxd environment %s\n%s\n", env.Name, env.WorkFile)
		return nil
	case "apply":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke ghxd apply <environment>")
		}
		return ghxd.Apply(ctx, args[1])
	default:
		return fmt.Errorf("unknown ghxd operation %q", args[0])
	}
}

func ghxdUsage() error {
	return fmt.Errorf("usage: smoke ghxd <show|bootstrap|apply> [environment]")
}
