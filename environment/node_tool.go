package environment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type NodeToolSpec struct {
	Name    string
	Package string
	Spec    string
}

type NodeTool struct {
	Name      string
	Package   string
	Spec      string
	Digest    string
	Root      string
	ModuleDir string
}

type nodeToolManifest struct {
	Name    string `json:"name"`
	Package string `json:"package"`
	Spec    string `json:"spec"`
	Digest  string `json:"digest"`
}

func InstallNodeTool(ctx context.Context, envName string, spec NodeToolSpec) (NodeTool, error) {
	env, err := Require(envName)
	if err != nil {
		return NodeTool{}, err
	}
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Package = strings.TrimSpace(spec.Package)
	spec.Spec = strings.TrimSpace(spec.Spec)
	if !validName.MatchString(spec.Name) {
		return NodeTool{}, fmt.Errorf("invalid node tool name %q", spec.Name)
	}
	if spec.Package == "" || spec.Spec == "" {
		return NodeTool{}, fmt.Errorf("node tool package and spec are required")
	}

	lock, err := acquireExclusive(ctx, env)
	if err != nil {
		return NodeTool{}, err
	}
	defer lock.Close()

	digestBytes := sha256.Sum256([]byte(spec.Package + "\x00" + spec.Spec))
	digest := hex.EncodeToString(digestBytes[:])
	storeRoot := filepath.Join(env.Dir, "node-tools", "store")
	root := filepath.Join(storeRoot, digest)
	moduleDir := filepath.Join(root, "node_modules", filepath.FromSlash(spec.Package))

	if _, err := os.Stat(filepath.Join(moduleDir, "package.json")); os.IsNotExist(err) {
		npm, lookupErr := exec.LookPath("npm")
		if lookupErr != nil {
			return NodeTool{}, fmt.Errorf("node tool %q requires npm: %w", spec.Name, lookupErr)
		}
		if err := os.MkdirAll(storeRoot, 0o700); err != nil {
			return NodeTool{}, err
		}
		tmp, err := os.MkdirTemp(storeRoot, ".install-*")
		if err != nil {
			return NodeTool{}, err
		}
		defer os.RemoveAll(tmp)

		packageJSON, err := json.MarshalIndent(map[string]any{
			"private": true,
			"dependencies": map[string]string{spec.Package: spec.Spec},
		}, "", "  ")
		if err != nil {
			return NodeTool{}, err
		}
		packageJSON = append(packageJSON, '\n')
		if err := os.WriteFile(filepath.Join(tmp, "package.json"), packageJSON, 0o600); err != nil {
			return NodeTool{}, err
		}

		cmd := exec.CommandContext(ctx, npm, "install", "--ignore-scripts", "--no-audit", "--no-fund")
		cmd.Dir = tmp
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return NodeTool{}, fmt.Errorf("install node tool %s: %w", spec.Name, err)
		}
		installed := filepath.Join(tmp, "node_modules", filepath.FromSlash(spec.Package), "package.json")
		if _, err := os.Stat(installed); err != nil {
			return NodeTool{}, fmt.Errorf("installed node tool %s: %w", spec.Name, err)
		}
		if err := os.Rename(tmp, root); err != nil {
			if _, statErr := os.Stat(filepath.Join(moduleDir, "package.json")); statErr != nil {
				return NodeTool{}, fmt.Errorf("commit node tool %s: %w", spec.Name, err)
			}
		}
	} else if err != nil {
		return NodeTool{}, err
	}

	manifest := nodeToolManifest{Name: spec.Name, Package: spec.Package, Spec: spec.Spec, Digest: digest}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return NodeTool{}, err
	}
	body = append(body, '\n')
	manifestPath := filepath.Join(env.Dir, "node-tools", spec.Name+".json")
	if err := writeAtomic(manifestPath, body); err != nil {
		return NodeTool{}, err
	}
	return NodeTool{Name: spec.Name, Package: spec.Package, Spec: spec.Spec, Digest: digest, Root: root, ModuleDir: moduleDir}, nil
}

func ResolveNodeTool(envName, toolName string) (NodeTool, bool, error) {
	env, err := Require(envName)
	if err != nil {
		return NodeTool{}, false, err
	}
	toolName = strings.TrimSpace(toolName)
	if !validName.MatchString(toolName) {
		return NodeTool{}, false, fmt.Errorf("invalid node tool name %q", toolName)
	}
	body, err := os.ReadFile(filepath.Join(env.Dir, "node-tools", toolName+".json"))
	if os.IsNotExist(err) {
		return NodeTool{}, false, nil
	}
	if err != nil {
		return NodeTool{}, false, err
	}
	var manifest nodeToolManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return NodeTool{}, false, fmt.Errorf("decode node tool %s: %w", toolName, err)
	}
	if manifest.Name != toolName || manifest.Package == "" || manifest.Spec == "" || manifest.Digest == "" {
		return NodeTool{}, false, fmt.Errorf("invalid node tool manifest %s", toolName)
	}
	root := filepath.Join(env.Dir, "node-tools", "store", manifest.Digest)
	moduleDir := filepath.Join(root, "node_modules", filepath.FromSlash(manifest.Package))
	if _, err := os.Stat(filepath.Join(moduleDir, "package.json")); err != nil {
		return NodeTool{}, false, fmt.Errorf("node tool %s payload: %w", toolName, err)
	}
	return NodeTool{Name: manifest.Name, Package: manifest.Package, Spec: manifest.Spec, Digest: manifest.Digest, Root: root, ModuleDir: moduleDir}, true, nil
}

func writeAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
