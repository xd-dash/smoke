// Package ghxdsecrets provides opt-in SOPS-backed GitHub device credentials.
// The original ghxd Go tool/worktree interface remains unchanged.
package ghxdsecrets

import (
 "context"
 "errors"
 "fmt"
 "os"
 "os/signal"
 "syscall"

 "github.com/xd-dash/smoke/command"
)

func init() { command.Register("ghxd-secrets", Run) }

func Run(args []string) error {
 if len(args) != 1 { return errors.New("usage: smoke ghxd-secrets <auth|status|probe>") }
 switch args[0] {
 case "auth":
  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer stop()
  return bootstrapGitHubToken(ctx)
 case "status":
  return checkEncryptedCredential(context.Background())
 case "probe":
  return probeCheckpoint(context.Background())
 default:
  return fmt.Errorf("unknown ghxd-secrets operation")
 }
}
