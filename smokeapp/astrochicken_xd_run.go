package smokeapp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xd-dash/smoke/command"
	"github.com/xd-dash/smoke/environment"
)

const (
	astrochickenXDRunEnvironment = "astrochicken-xd-run"
	agniProbeToolPath            = "github.com/dash-xd/agni/cmd/probe"
	cfxdDNSTXTToolPath           = "github.com/xd-dash/smoke/cmd/cfxd-dns-txt"
)

var exactGitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func init() {
	command.Register("astrochicken-xd-run", runAstrochickenXDRun)
}

func runAstrochickenXDRun(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return astrochickenXDRunUsage()
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return astrochickenXDRunUsage()
		}
		fmt.Printf("environment\t%s\n", astrochickenXDRunEnvironment)
		fmt.Printf("profile\tagni/probe\t%s\n", agniProbeToolPath)
		fmt.Printf("profile\tcfxd/dns-txt\t%s\n", cfxdDNSTXTToolPath)
		fmt.Println("config\tprobe.tfvars")
		fmt.Println("config\txd-run.routes")
		fmt.Println("terraform-state\tagni-probe\tindependent")
		fmt.Println("terraform-state\tcfxd-dns-txt\tindependent")
		return nil
	case "bootstrap":
		agniSHA, cfxdSHA, err := parseProfileSHAs(args[1:])
		if err != nil {
			return err
		}
		env, err := bootstrapAstrochickenXDRun(ctx, agniSHA, cfxdSHA)
		if err != nil {
			return err
		}
		fmt.Printf("astrochicken-xd-run environment %s\n%s\n", env.Name, env.WorkFile)
		return nil
	case "seed":
		root, probeVars, routes, err := parseCompositionSeed(args[1:])
		if err != nil {
			return err
		}
		return seedAstrochickenXDRun(ctx, root, probeVars, routes)
	default:
		return fmt.Errorf("unknown astrochicken-xd-run operation %q", args[0])
	}
}

func bootstrapAstrochickenXDRun(ctx context.Context, agniSHA, cfxdSHA string) (environment.Environment, error) {
	if !exactGitSHA.MatchString(agniSHA) {
		return environment.Environment{}, fmt.Errorf("agni SHA must be an exact 40-character Git SHA")
	}
	if !exactGitSHA.MatchString(cfxdSHA) {
		return environment.Environment{}, fmt.Errorf("cfxd SHA must be an exact 40-character Git SHA")
	}

	env, err := environment.Require(astrochickenXDRunEnvironment)
	if err != nil {
		env, err = environment.Create(ctx, astrochickenXDRunEnvironment)
		if err != nil {
			return environment.Environment{}, err
		}
	}
	for _, spec := range []string{
		agniProbeToolPath + "@" + strings.ToLower(agniSHA),
		cfxdDNSTXTToolPath + "@" + strings.ToLower(cfxdSHA),
	} {
		if err := environment.AddTool(ctx, env.Name, spec); err != nil {
			return environment.Environment{}, fmt.Errorf("compose profile tool %s: %w", spec, err)
		}
	}
	return env, nil
}

func seedAstrochickenXDRun(ctx context.Context, root, probeVars, routes string) error {
	env, err := environment.Require(astrochickenXDRunEnvironment)
	if err != nil {
		return fmt.Errorf("astrochicken-xd-run is not bootstrapped: %w", err)
	}
	workspace, err := environment.Snapshot(ctx, env)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("astrochicken-xd-run requires a preinstalled Go toolchain: %w", err)
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	for _, invocation := range [][]string{
		{"tool", "probe", "seed", filepath.Join(root, "agni-probe")},
		{"tool", "cfxd-dns-txt", "seed", filepath.Join(root, "cfxd-dns-txt")},
	} {
		cmd := workspace.Command(ctx, root, goBin, invocation...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(invocation[:2], " "), err)
		}
	}

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if err := copyCompositionConfig(probeVars, filepath.Join(configDir, "probe.tfvars")); err != nil {
		return fmt.Errorf("copy probe.tfvars: %w", err)
	}
	if err := copyCompositionConfig(routes, filepath.Join(configDir, "xd-run.routes")); err != nil {
		return fmt.Errorf("copy xd-run.routes: %w", err)
	}
	return nil
}

func copyCompositionConfig(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func parseProfileSHAs(args []string) (string, string, error) {
	var agniSHA, cfxdSHA string
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", astrochickenXDRunBootstrapUsage()
		}
		switch args[0] {
		case "--agni-sha":
			agniSHA = args[1]
		case "--cfxd-sha":
			cfxdSHA = args[1]
		default:
			return "", "", astrochickenXDRunBootstrapUsage()
		}
		args = args[2:]
	}
	if agniSHA == "" || cfxdSHA == "" {
		return "", "", astrochickenXDRunBootstrapUsage()
	}
	return agniSHA, cfxdSHA, nil
}

func parseCompositionSeed(args []string) (string, string, string, error) {
	if len(args) < 1 {
		return "", "", "", astrochickenXDRunSeedUsage()
	}
	root := args[0]
	args = args[1:]
	var probeVars, routes string
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", "", astrochickenXDRunSeedUsage()
		}
		switch args[0] {
		case "--probe-vars":
			probeVars = args[1]
		case "--routes":
			routes = args[1]
		default:
			return "", "", "", astrochickenXDRunSeedUsage()
		}
		args = args[2:]
	}
	if probeVars == "" || routes == "" {
		return "", "", "", astrochickenXDRunSeedUsage()
	}
	return root, probeVars, routes, nil
}

func astrochickenXDRunUsage() error {
	return fmt.Errorf("usage: smoke astrochicken-xd-run <show|bootstrap|seed> ...")
}

func astrochickenXDRunBootstrapUsage() error {
	return fmt.Errorf("usage: smoke astrochicken-xd-run bootstrap --agni-sha <40-char-sha> --cfxd-sha <40-char-sha>")
}

func astrochickenXDRunSeedUsage() error {
	return fmt.Errorf("usage: smoke astrochicken-xd-run seed <root> --probe-vars <probe.tfvars> --routes <xd-run.routes>")
}
