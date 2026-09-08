// Package cli provides the command-line composition surface shared by the
// canonical Smoke executable and generated self-composition programs.
//
// Domain behavior belongs in reusable packages such as environment, ghxd, and
// logmash. This package is intentionally limited to command dispatch, argument
// adaptation, and child-process wiring.
package cli
