// Package astrochicken defines Smoke's minimal domain recipe for probing a
// representative Agni regional composition. Agni itself does not know the
// Astrochicken name or gateway/world semantics.
package astrochicken

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	smokeagni "github.com/xd-dash/smoke/agni"
)

var modules = []string{
	"regional-network",
	"regional-internal-addresses",
	"coreos-node",
	"regional-cell",
	"cloud-function-v1-http",
	"cloud-function-v2-http",
}

// Run executes the Astrochicken lifecycle against the single Agni provider
// compiled into this Smoke composition. Terraform configuration lives in
// astrochicken/terraform; Go only selects the generic Agni modules and invokes
// the provider with those ordinary HCL root files.
func Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smoke agni astrochicken <deploy|plan|destroy|output> [terraform-args...]")
	}

	verb := strings.TrimSpace(args[0])
	if verb == "" {
		return fmt.Errorf("astrochicken verb is required")
	}

	provider, err := smokeagni.DefaultProvider()
	if err != nil {
		return err
	}
	scope := smokeagni.CurrentScope()
	if scope.Kind == smokeagni.ScopeEnvironment && verb != "output" {
		return fmt.Errorf("astrochicken %s is not available inside Smoke environment %q; environment scope is observational", verb, scope.Environment)
	}

	workspace, err := workspaceDir(scope)
	if err != nil {
		return err
	}
	files, err := terraformFiles()
	if err != nil {
		return err
	}
	req := smokeagni.TerraformRequest{
		Scope:     scope,
		Workspace: workspace,
		Modules:   append([]string(nil), modules...),
		Files:     files,
	}

	switch verb {
	case "deploy":
		if err := runTerraform(ctx, provider, req, []string{"init"}); err != nil {
			return err
		}
		return runTerraform(ctx, provider, req, append([]string{"apply"}, args[1:]...))
	case "plan":
		if err := runTerraform(ctx, provider, req, []string{"init"}); err != nil {
			return err
		}
		return runTerraform(ctx, provider, req, append([]string{"plan"}, args[1:]...))
	case "destroy":
		if err := runTerraform(ctx, provider, req, []string{"init"}); err != nil {
			return err
		}
		return runTerraform(ctx, provider, req, append([]string{"destroy"}, args[1:]...))
	case "output":
		return runTerraform(ctx, provider, req, append([]string{"output"}, args[1:]...))
	default:
		return fmt.Errorf("unknown astrochicken verb %q", verb)
	}
}

func runTerraform(ctx context.Context, provider smokeagni.Provider, req smokeagni.TerraformRequest, args []string) error {
	req.Args = append([]string(nil), args...)
	return provider.RunTerraform(ctx, req)
}

func workspaceDir(scope smokeagni.Scope) (string, error) {
	if override := strings.TrimSpace(os.Getenv("SMOKE_ASTROCHICKEN_WORKSPACE")); override != "" {
		return filepath.Abs(override)
	}
	if scope.Kind == smokeagni.ScopeEnvironment {
		base := strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
		if base == "" {
			return "", fmt.Errorf("SMOKE_ENV_WORKSPACE is required in environment scope")
		}
		return filepath.Join(base, "agni", "astrochicken"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".smoke", "agni", "astrochicken"), nil
}
