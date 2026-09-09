// Package astrochickenxdrun defines the named Smoke composition that combines
// Agni Probe with cfxd dns-txt while preserving independent Terraform roots.
package astrochickenxdrun

import (
	"context"
	"fmt"
	"strings"

	"github.com/xd-dash/smoke/environment"
)

const DefaultEnvironment = "astrochicken-xd-run"

const (
	agniProbeToolSpec = "github.com/dash-xd/agni/cmd/probe@cc2a201d2271de1974bd59a78f9066cffefe940b"
	cfxdDNSTXTToolSpec = "github.com/xd-dash/smoke/cmd/cfxd-dns-txt@92ac05da5f162beaf54b599242ebd14a06892719"
)

var ToolSpecs = []string{
	agniProbeToolSpec,
	cfxdDNSTXTToolSpec,
}

// Apply composes the exact profile tools into an existing Smoke environment.
// It does not seed or couple their Terraform roots.
func Apply(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("environment name is required")
	}
	for _, spec := range ToolSpecs {
		if err := environment.AddTool(ctx, name, spec); err != nil {
			return fmt.Errorf("add astrochicken-xd-run tool %s: %w", spec, err)
		}
	}
	return nil
}

// Bootstrap ensures the named environment exists and contains the exact
// profile tools. Re-running it updates through Go's native tool dependency
// mechanism, matching ghxd's idempotent bootstrap model.
func Bootstrap(ctx context.Context, name string) (environment.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultEnvironment
	}

	env, err := environment.Require(name)
	if err != nil {
		env, err = environment.Create(ctx, name)
		if err != nil {
			return environment.Environment{}, err
		}
	}
	if err := Apply(ctx, env.Name); err != nil {
		return environment.Environment{}, err
	}
	return env, nil
}
