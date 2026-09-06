package smokeapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xd-dash/smoke/environment"
	"github.com/xd-dash/smoke/identity"
)

const inspectionSchema = 1

type runtimeInspectionDocument struct {
	Schema      int                    `json:"schema"`
	Kind        string                 `json:"kind"`
	Composition runtimeCompositionJSON `json:"composition"`
	Runtime     runtimeContextJSON     `json:"runtime"`
}

type runtimeCompositionJSON struct {
	Digest     string   `json:"digest"`
	Executable string   `json:"executable"`
	GoVersion  string   `json:"go_version"`
	Components []string `json:"components"`
}

type runtimeContextJSON struct {
	Environment     string `json:"environment"`
	WorkspaceDigest string `json:"workspace_digest"`
	Workspace       string `json:"workspace"`
}

type environmentInspectionDocument struct {
	Schema          int                     `json:"schema"`
	Kind            string                  `json:"kind"`
	Environment     environmentIdentityJSON `json:"environment"`
	RuntimeSnapshot workspaceIdentityJSON   `json:"runtime_snapshot"`
}

type environmentIdentityJSON struct {
	Name           string `json:"name"`
	CanonicalWork  string `json:"canonical_work"`
	CanonicalTools string `json:"canonical_tools"`
}

type workspaceIdentityJSON struct {
	Digest string `json:"digest"`
	Work   string `json:"work"`
	Tools  string `json:"tools"`
}

func inspectRuntime(args []string) error {
	jsonOutput, err := parseJSONOnlyFlag(args, "smoke inspect [--json]")
	if err != nil {
		return err
	}
	doc := runtimeInspection()
	if jsonOutput {
		return writeJSON(doc)
	}

	fmt.Println("Smoke composition")
	fmt.Printf("  digest: %s\n", doc.Composition.Digest)
	fmt.Printf("  executable: %s\n", doc.Composition.Executable)
	fmt.Printf("  go: %s\n", doc.Composition.GoVersion)
	fmt.Println("  components:")
	if len(doc.Composition.Components) == 0 {
		fmt.Println("    (none registered)")
	} else {
		for _, component := range doc.Composition.Components {
			fmt.Printf("    %s\n", component)
		}
	}

	fmt.Println("Runtime")
	fmt.Printf("  environment: %s\n", displayNone(doc.Runtime.Environment))
	fmt.Printf("  workspace digest: %s\n", displayNone(doc.Runtime.WorkspaceDigest))
	fmt.Printf("  workspace: %s\n", displayNone(doc.Runtime.Workspace))
	return nil
}

func runtimeInspection() runtimeInspectionDocument {
	info := identity.Current()
	envName := strings.TrimSpace(os.Getenv("SMOKE_ENV"))
	workspace := strings.TrimSpace(os.Getenv("SMOKE_ENV_WORKSPACE"))
	if workspace == "" {
		workspace = strings.TrimSpace(os.Getenv("GOWORK"))
	}
	return runtimeInspectionDocument{
		Schema: inspectionSchema,
		Kind:   "smoke.runtime",
		Composition: runtimeCompositionJSON{
			Digest:     info.Digest,
			Executable: info.Executable,
			GoVersion:  info.GoVersion,
			Components: info.Components,
		},
		Runtime: runtimeContextJSON{
			Environment:     envName,
			WorkspaceDigest: identity.WorkspaceDigest(),
			Workspace:       workspace,
		},
	}
}

func inspectEnvironment(ctx context.Context, args []string) error {
	name, jsonOutput, err := parseEnvironmentInspectArgs(args)
	if err != nil {
		return err
	}
	doc, err := environmentInspection(ctx, name)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(doc)
	}

	fmt.Printf("Environment %s\n", doc.Environment.Name)
	fmt.Printf("  canonical work: %s\n", doc.Environment.CanonicalWork)
	fmt.Printf("  canonical tools: %s\n", doc.Environment.CanonicalTools)
	fmt.Println("Runtime snapshot")
	fmt.Printf("  digest: %s\n", doc.RuntimeSnapshot.Digest)
	fmt.Printf("  work: %s\n", doc.RuntimeSnapshot.Work)
	fmt.Printf("  tools: %s\n", doc.RuntimeSnapshot.Tools)
	return nil
}

func environmentInspection(ctx context.Context, name string) (environmentInspectionDocument, error) {
	env, err := environment.Require(name)
	if err != nil {
		return environmentInspectionDocument{}, err
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return environmentInspectionDocument{}, err
	}
	return environmentInspectionDocument{
		Schema: inspectionSchema,
		Kind:   "smoke.environment",
		Environment: environmentIdentityJSON{
			Name:           env.Name,
			CanonicalWork:  env.WorkFile,
			CanonicalTools: filepath.Join(env.ToolsDir, "go.mod"),
		},
		RuntimeSnapshot: workspaceIdentityJSON{
			Digest: workspace.Digest,
			Work:   workspace.WorkFile,
			Tools:  filepath.Join(workspace.ToolsDir, "go.mod"),
		},
	}, nil
}

func parseJSONOnlyFlag(args []string, usage string) (bool, error) {
	switch {
	case len(args) == 0:
		return false, nil
	case len(args) == 1 && args[0] == "--json":
		return true, nil
	default:
		return false, fmt.Errorf("usage: %s", usage)
	}
}

func parseEnvironmentInspectArgs(args []string) (string, bool, error) {
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" && args[0] != "--json" {
		return args[0], false, nil
	}
	if len(args) == 2 {
		if args[0] == "--json" && strings.TrimSpace(args[1]) != "" {
			return args[1], true, nil
		}
		if args[1] == "--json" && strings.TrimSpace(args[0]) != "" {
			return args[0], true, nil
		}
	}
	return "", false, fmt.Errorf("usage: smoke env inspect <name> [--json]")
}

func writeJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(v)
}

func displayNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(none)"
	}
	return value
}
