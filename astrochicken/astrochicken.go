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
	"coreos-node",
	"regional-cell",
	"cloud-function-v1-http",
	"cloud-function-v2-http",
}

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

locals {
  function_invoker_members = distinct(concat(
    var.function_invoker_members,
    var.service_account_email == "" ? [] : ["serviceAccount:${var.service_account_email}"]
  ))
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
  private_ip_google_access = true

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

module "gen1_functions" {
  for_each = var.gen1_functions
  source   = "./modules/cloud-function-v1-http"

  project               = var.project
  region                = var.region
  name                  = each.key
  runtime               = each.value.runtime
  entry_point           = each.value.entry_point
  source_archive_bucket = each.value.source_archive_bucket
  source_archive_object = each.value.source_archive_object
  ingress_settings      = "ALLOW_INTERNAL_ONLY"
  invoker_members       = local.function_invoker_members
  service_account_email = each.value.service_account_email
  available_memory_mb   = each.value.available_memory_mb
  timeout_seconds       = each.value.timeout_seconds
  environment_variables = each.value.environment_variables
  labels                = merge({ "smoke-role" = "shadow" }, each.value.labels)
}

module "gen2_functions" {
  for_each = var.gen2_functions
  source   = "./modules/cloud-function-v2-http"

  project               = var.project
  region                = var.region
  name                  = each.key
  runtime               = each.value.runtime
  entry_point           = each.value.entry_point
  source_archive_bucket = each.value.source_archive_bucket
  source_archive_object = each.value.source_archive_object
  ingress_settings      = "ALLOW_INTERNAL_ONLY"
  invoker_members       = local.function_invoker_members
  service_account_email = each.value.service_account_email
  available_memory      = each.value.available_memory
  timeout_seconds       = each.value.timeout_seconds
  min_instance_count    = each.value.min_instance_count
  max_instance_count    = each.value.max_instance_count
  environment_variables = each.value.environment_variables
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
    condition     = can(cidrhost(var.ipv4_cidr, 0)) && tonumber(split("/", var.ipv4_cidr)[1]) == 29
    error_message = "Astrochicken ipv4_cidr must be a valid IPv4 /29 CIDR."
  }
}
variable "source_instance_template" { type = string }
variable "service_account_email" {
  type    = string
  default = ""
}
variable "function_invoker_members" {
  type    = list(string)
  default = []
}
variable "gen1_functions" {
  type = map(object({
    runtime               = string
    entry_point           = string
    source_archive_bucket = string
    source_archive_object = string
    service_account_email = optional(string, "")
    available_memory_mb   = optional(number, 256)
    timeout_seconds       = optional(number, 60)
    environment_variables = optional(map(string), {})
    labels                = optional(map(string), {})
  }))
  default = {}
}
variable "gen2_functions" {
  type = map(object({
    runtime               = string
    entry_point           = string
    source_archive_bucket = string
    source_archive_object = string
    service_account_email = optional(string, "")
    available_memory      = optional(string, "256M")
    timeout_seconds       = optional(number, 60)
    min_instance_count    = optional(number, 0)
    max_instance_count    = optional(number, 1)
    environment_variables = optional(map(string), {})
  }))
  default = {}
}
`),
	"outputs.tf": []byte(`output "subnetwork" { value = module.region.subnetwork }
output "nodes" { value = module.region.nodes }
output "spare_ipv4_by_slot" {
  value = {
    "2" = cidrhost(var.ipv4_cidr, 4)
    "3" = cidrhost(var.ipv4_cidr, 5)
  }
}
output "shadow_functions" {
  value = {
    gen1 = { for name, fn in module.gen1_functions : name => fn.uri }
    gen2 = { for name, fn in module.gen2_functions : name => fn.uri }
  }
}
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
