package smokeapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/identity"
)

func inspectRuntime() error {
	info := identity.Current()
	fmt.Println("Smoke composition")
	fmt.Printf("  digest: %s\n", info.Digest)
	fmt.Printf("  executable: %s\n", info.Executable)
	fmt.Printf("  go: %s\n", info.GoVersion)
	fmt.Println("  components:")
	if len(info.Components) == 0 {
		fmt.Println("    (none registered)")
	} else {
		for _, component := range info.Components {
			fmt.Printf("    %s\n", component)
		}
	}

	fmt.Println("Runtime")
	envName := strings.TrimSpace(os.Getenv("SMOKE_ENV"))
	workspace := strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
	if envName == "" && workspace == "" {
		fmt.Println("  environment: (none)")
		fmt.Println("  workspace digest: (none)")
		fmt.Println("  workspace: (none)")
		return nil
	}
	if envName == "" {
		envName = "(unknown)"
	}
	fmt.Printf("  environment: %s\n", envName)
	digest := identity.WorkspaceDigest()
	if digest == "" {
		digest = "(none)"
	}
	if workspace == "" {
		workspace = strings.TrimSpace(os.Getenv("GOWORK"))
	}
	if workspace == "" {
		workspace = "(none)"
	}
	fmt.Printf("  workspace digest: %s\n", digest)
	fmt.Printf("  workspace: %s\n", workspace)
	return nil
}

func inspectEnvironment(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: smoke env inspect <name>")
	}
	env, err := environment.Require(args[0])
	if err != nil {
		return err
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return err
	}
	fmt.Printf("Environment %s\n", env.Name)
	fmt.Printf("  canonical work: %s\n", env.WorkFile)
	fmt.Printf("  canonical tools: %s\n", filepath.Join(env.ToolsDir, "go.mod"))
	fmt.Println("Runtime snapshot")
	fmt.Printf("  digest: %s\n", workspace.Digest)
	fmt.Printf("  work: %s\n", workspace.WorkFile)
	fmt.Printf("  tools: %s\n", filepath.Join(workspace.ToolsDir, "go.mod"))
	return nil
}
