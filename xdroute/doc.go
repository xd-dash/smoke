// Package xdroute defines Smoke's provider-neutral DNS route contract.
//
// A route identity is encoded in the DNS owner name:
//
//	_<provider>._<role>.<service>.<zone>
//
// Route placement is encoded in TXT content:
//
//	xd-route=v1;region=<region>;edge=<edge>;host=<host>
//
// Region and edge are optional. Host is required. Multiple xd-route TXT
// records at the same owner are alternatives for the same logical identity;
// DNS record order does not imply preference.
//
// This package intentionally performs no DNS or provider API calls. DNS
// providers such as Cloudflare adapters, Terraform integrations, and runtime
// resolvers can share the same encoding without depending on one another.
package xdroute
