// Package dnstxt owns the cfxd DNS TXT infrastructure profile.
package dnstxt

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed terraform/*.tf
var profileFS embed.FS

// Seed writes the complete dns-txt Terraform root into dst. Profile source and
// deployment configuration intentionally remain separate: Seed never embeds or
// writes concrete xdroute values.
//
// Root-level *.tf files are profile-owned. Reseeding reconciles that source set
// so Terraform cannot continue loading a file removed from the selected profile.
// Runtime/provider artifacts such as .terraform/, tfstate, and generated tfvars
// are not profile source and are left untouched.
func Seed(dst string) error {
	dst = strings.TrimSpace(dst)
	if dst == "" {
		return fmt.Errorf("destination is required")
	}

	root, err := fs.Sub(profileFS, "terraform")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := reconcileTerraformSource(root, dst); err != nil {
		return err
	}

	return fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func reconcileTerraformSource(root fs.FS, dst string) error {
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return err
	}
	desired := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tf" {
			desired[entry.Name()] = struct{}{}
		}
	}

	existing, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	for _, entry := range existing {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}
		if _, ok := desired[entry.Name()]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dst, entry.Name())); err != nil {
			return fmt.Errorf("remove stale profile source %s: %w", entry.Name(), err)
		}
	}
	return nil
}
