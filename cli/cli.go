package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/xd-dash/smoke/command"
)

// Main runs the Smoke command-line application and terminates the process on
// error. Composition programs normally call Main from their package main.
func Main(args []string) {
	if err := Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(1)
	}
}

// Run dispatches one Smoke invocation without terminating the process.
func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "commands":
		for _, name := range command.Names() {
			fmt.Println(name)
		}
		return nil
	case "inspect":
		return inspectRuntime(args[1:])
	case "compose":
		return runCompose(args[1:])
	case "env":
		return runEnv(args[1:])
	default:
		return command.Run(args[0], args[1:])
	}
}

func usageError() error {
	names := command.Names()
	if len(names) == 0 {
		return fmt.Errorf("usage: smoke inspect | smoke <command> [args ...] | smoke compose <show|add|remove|rebuild> | smoke env <operation> ...")
	}
	return fmt.Errorf(
		"usage: smoke inspect | smoke <%s> [args ...] | smoke compose <show|add|remove|rebuild> | smoke env <operation> ...",
		strings.Join(names, "|"),
	)
}
