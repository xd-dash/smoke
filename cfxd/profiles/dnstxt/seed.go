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
func Seed(dst string) error {
	dst = strings.TrimSpace(dst)
	if dst == "" {
		return fmt.Errorf("destination is required")
	}

	root, err := fs.Sub(profileFS, "terraform")
	if err != nil {
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
