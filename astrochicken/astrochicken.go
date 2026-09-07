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

var modules = []string{"regional-network", "coreos-node", "regional-cell"}

var rootFiles = map[string][]byte{
	"main.tf": []byte(`terraform {
  required_version = ">= 1.3.0"

  required_providers {
    google = {
      source = "hashicorp/google"
    }
  }
}

provider "google" {
  project = var.project
  region  = var.region
  zone    = var.zone
}

module "region" {
  source = "./modules/regional-cell"

  project                  = var.project
  region                   = var.region
  zone                     = var.zone
  network                  = var.network
  subnetwork_name          = var.subnetwork_name
  ipv4_cidr                = var.ipv4_cidr
  source_instance_template = var.source_instance_template
  enable_ipv6              = true

  nodes = {
    "0" = {
      metadata = { "smoke-role" = "gateway" }
      service_account_email = var.service_account_email
    }
    "1" = {
      metadata = { "smoke-role" = "world" }
      service_account_email = var.service_account_email
    }
  }
}
`),
	"variables.tf": []byte(`variable "project" { type = string }
variable "region" { type = string }
variable "zone" { type = string }
variable "network" { type = string }
variable "subnetwork_name" { type = string }
variable "ipv4_cidr" {
  type = string
  validation {
    condition     = can(cidrhost(var.ipv4_cidr, 0)) && tonumber(split("/", var.ipv4_cidr)[1]) == 28
    error_message = "ipv4_cidr must be a valid IPv4 /28 CIDR."
  }
}
variable "source_instance_template" { type = string }
variable "service_account_email" {
  type    = string
  default = ""
}
`),
	"outputs.tf": []byte(`output "subnetwork" { value = module.region.subnetwork }
output "nodes" { value = module.region.nodes }
`),
}

// Run executes the Astrochicken lifecycle against the single Agni provider
// compiled into this Smoke composition.
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
	req := smokeagni.TerraformRequest{
		Scope:     scope,
		Workspace: workspace,
		Modules:   append([]string(nil), modules...),
		Files:     cloneFiles(rootFiles),
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

func cloneFiles(src map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(src))
	for name, data := range src {
		out[name] = append([]byte(nil), data...)
	}
	return out
}
