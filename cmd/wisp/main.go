// Package wisp registers the optional Wisp command in a Smoke composition.
package wisp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/xd-dash/smoke/command"
	transport "github.com/xd-dash/smoke/wisp"
)

func init() { command.Register("wisp", Run) }

func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: smoke wisp exec|forward|qualify [options]")
	}
	action := args[0]
	flags := flag.NewFlagSet("wisp "+action, flag.ContinueOnError)
	var config transport.Config
	flags.StringVar(&config.Address, "address", "127.0.0.1:2222", "SSH endpoint")
	flags.StringVar(&config.User, "user", "wisp", "SSH user")
	flags.StringVar(&config.IdentityFile, "identity", "", "private key file")
	flags.StringVar(&config.KnownHostsFile, "known-hosts", "", "existing trusted OpenSSH known_hosts file")
	timeout := flags.Duration("timeout", 2*time.Minute, "operation deadline")
	listen := flags.String("listen", "", "absolute local Unix socket path")
	network := flags.String("target-network", "tcp", "remote tcp or unix")
	target := flags.String("target", "127.0.0.1:16379", "remote endpoint")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	base, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(base, *timeout)
	defer cancel()
	switch action {
	case "exec":
		if len(flags.Args()) == 0 {
			return errors.New("exec requires a command after --")
		}
		quoted := make([]string, len(flags.Args()))
		for i, arg := range flags.Args() {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
		return config.Exec(ctx, strings.Join(quoted, " "), os.Stdin, os.Stdout, os.Stderr)
	case "forward":
		if flags.NArg() != 0 {
			return errors.New("unexpected forward arguments")
		}
		var listener net.Listener
		var err error
		listener, err = transport.ListenUnix(*listen)
		if err != nil {
			return err
		}
		err = config.Forward(ctx, listener, *network, *target, func() { fmt.Println("WISP_FORWARD_READY=true") })
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	case "qualify":
		if flags.NArg() != 0 {
			return errors.New("unexpected qualify arguments")
		}
		return config.Exec(ctx, "bash -se", strings.NewReader(qualification), os.Stdout, os.Stderr)
	default:
		return fmt.Errorf("unknown Wisp action %q", action)
	}
}

const qualification = `set -Eeuo pipefail
test "$(id -u)" -ne 0
test -S "$XDG_RUNTIME_DIR/bus"
systemctl --user show-environment >/dev/null
test "$(podman info --format '{{.Host.Security.Rootless}}')" = true
unit="wisp-go-$(cat /proc/sys/kernel/random/uuid)"
cleanup() {
 systemctl --user stop "$unit.service" "$unit-fail.service" "$unit-timeout.service" >/dev/null 2>&1 || true
 systemctl --user reset-failed "$unit.service" "$unit-fail.service" "$unit-timeout.service" >/dev/null 2>&1 || true
}
trap cleanup EXIT
systemd-run --user --wait --pipe --unit="$unit" --property=RuntimeMaxSec=30s /usr/bin/bash -c 'test -n "$INVOCATION_ID"; test "$(id -u)" -ne 0; podman info >/dev/null'
if systemd-run --user --wait --pipe --unit="$unit-fail" /usr/bin/false; then exit 1; fi
test "$(systemctl --user show "$unit-fail.service" -p ExecMainStatus --value)" = 1
if systemd-run --user --wait --pipe --unit="$unit-timeout" --property=RuntimeMaxSec=2s /usr/bin/sleep 20; then exit 1; fi
test "$(systemctl --user show "$unit-timeout.service" -p Result --value)" = timeout
echo WISP_SYSTEMD_RESULT=passed
`
