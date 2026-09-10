package environment

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallAndResolveNodeTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake npm fixture uses a POSIX shell")
	}

	t.Setenv("SMOKE_ENV_DIR", t.TempDir())
	env, err := Create(context.Background(), "node-tools")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok, err := ResolveNodeTool(env.Name, "probot-runtime"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("tool unexpectedly present before installation")
	}

	bin := t.TempDir()
	npm := filepath.Join(bin, "npm")
	fixture := "#!/bin/sh\nset -eu\nmkdir -p node_modules/@xd-dash/probot-runtime\nprintf '{}\\n' > node_modules/@xd-dash/probot-runtime/package.json\n"
	if err := os.WriteFile(npm, []byte(fixture), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	spec := NodeToolSpec{
		Name:    "probot-runtime",
		Package: "@xd-dash/probot-runtime",
		Spec:    "github:xd-dash/probot-runtime#0123456789012345678901234567890123456789",
	}
	installed, err := InstallNodeTool(context.Background(), env.Name, spec)
	if err != nil {
		t.Fatal(err)
	}
	if installed.ModuleDir == "" || installed.Digest == "" {
		t.Fatalf("installed tool missing immutable identity: %#v", installed)
	}

	resolved, ok, err := ResolveNodeTool(env.Name, spec.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("installed tool was not discoverable")
	}
	if resolved.Digest != installed.Digest || resolved.ModuleDir != installed.ModuleDir {
		t.Fatalf("resolved tool = %#v, installed = %#v", resolved, installed)
	}
}
