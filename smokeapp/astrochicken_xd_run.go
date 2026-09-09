package smokeapp

import (
	"context"
	"fmt"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/composition/astrochickenxdrun"
)

func init() {
	command.Register("astrochicken-xd-run", runAstrochickenXDRun)
}

func runAstrochickenXDRun(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return astrochickenXDRunUsage()
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("usage: smoke astrochicken-xd-run show")
		}
		fmt.Printf("environment\t%s\n", astrochickenxdrun.DefaultEnvironment)
		for _, spec := range astrochickenxdrun.ToolSpecs {
			fmt.Printf("profile-tool\t%s\n", spec)
		}
		fmt.Println("terraform-root\tagni-probe")
		fmt.Println("terraform-root\tcfxd-dns-txt")
		fmt.Println("config\tconfig/probe.tfvars")
		fmt.Println("config\tconfig/xd-run.routes")
		return nil
	case "bootstrap":
		if len(args) > 2 {
			return fmt.Errorf("usage: smoke astrochicken-xd-run bootstrap [workspace-root]")
		}
		env, err := astrochickenxdrun.Bootstrap(ctx, "")
		if err != nil {
			return err
		}
		fmt.Printf("astrochicken-xd-run environment %s\n%s\n", env.Name, env.WorkFile)
		if len(args) == 2 {
			if err := astrochickenxdrun.SeedWorkspace(ctx, env.Name, args[1]); err != nil {
				return err
			}
			fmt.Printf("workspace %s\n", args[1])
		}
		return nil
	case "seed":
		if len(args) != 2 {
			return fmt.Errorf("usage: smoke astrochicken-xd-run seed <workspace-root>")
		}
		if err := astrochickenxdrun.SeedWorkspace(ctx, astrochickenxdrun.DefaultEnvironment, args[1]); err != nil {
			return err
		}
		fmt.Printf("workspace %s\n", args[1])
		return nil
	default:
		return fmt.Errorf("unknown astrochicken-xd-run operation %q", args[0])
	}
}

func astrochickenXDRunUsage() error {
	return fmt.Errorf("usage: smoke astrochicken-xd-run <show|bootstrap|seed> ...")
}
