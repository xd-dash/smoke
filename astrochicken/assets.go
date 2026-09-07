package astrochicken

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// terraformRoot contains the Astrochicken-owned Terraform root as ordinary
// HCL files. Go only materializes these files for the selected Agni provider;
// it does not construct Terraform configuration as string literals.
//
//go:embed terraform/*.tf
var terraformRoot embed.FS

func terraformFiles() (map[string][]byte, error) {
	root, err := fs.Sub(terraformRoot, "terraform")
	if err != nil {
		return nil, fmt.Errorf("open Astrochicken Terraform root: %w", err)
	}

	files := map[string][]byte{}
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
		files[filepath.ToSlash(path)] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read Astrochicken Terraform root: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("Astrochicken Terraform root is empty")
	}
	return files, nil
}
