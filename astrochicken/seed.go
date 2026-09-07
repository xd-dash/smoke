// Package astrochicken exposes the Astrochicken Terraform recipe as ordinary
// source. It is an optional environment recipe, not a compiled-in Smoke
// command, provider, or infrastructure implementation.
package astrochicken

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed terraform/*.tf
var terraformRoot embed.FS

// Seed copies the exact Astrochicken Terraform recipe into dst. It performs no
// Git/worktree operations and does not invoke or reinterpret Terraform.
func Seed(dst string) error {
	dst = strings.TrimSpace(dst)
	if dst == "" {
		return fmt.Errorf("destination is required")
	}
	root, err := fs.Sub(terraformRoot, "terraform")
	if err != nil {
		return fmt.Errorf("open Astrochicken Terraform root: %w", err)
	}
	count := 0
	err = fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".tf" {
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
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed Astrochicken Terraform root: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("Astrochicken Terraform root is empty")
	}
	return nil
}
