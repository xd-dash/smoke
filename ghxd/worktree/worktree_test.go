package worktree

import (
	"context"
	"strings"
	"testing"
)

func TestSeedValidation(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{name: "repository", opts: Options{Repository: "bad", SHA: strings.Repeat("a", 40), Destination: t.TempDir() + "/out"}, want: "repository must be owner/name"},
		{name: "sha", opts: Options{Repository: "xd-dash/smoke", SHA: "main", Destination: t.TempDir() + "/out"}, want: "sha must be an exact 40-character commit"},
		{name: "role ref dash", opts: Options{Repository: "xd-dash/smoke", SHA: strings.Repeat("a", 40), RoleRef: "-bad", Destination: t.TempDir() + "/out"}, want: "role ref must not begin"},
		{name: "role ref newline", opts: Options{Repository: "xd-dash/smoke", SHA: strings.Repeat("a", 40), RoleRef: "main\nother", Destination: t.TempDir() + "/out"}, want: "role ref must be one line"},
		{name: "destination", opts: Options{Repository: "xd-dash/smoke", SHA: strings.Repeat("a", 40)}, want: "destination is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Seed(context.Background(), tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestGitAuthArgsDoesNotExposeRawToken(t *testing.T) {
	token := "secret-token"
	args := gitAuthArgs(token)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, token) {
		t.Fatalf("auth args expose raw token: %q", joined)
	}
	redacted := strings.Join(redactArgs(args), " ")
	if strings.Contains(redacted, "AUTHORIZATION: basic") {
		t.Fatalf("redacted args still expose authorization header: %q", redacted)
	}
}

func TestGitAuthEnvIsolatesAmbientGitConfig(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/tmp/runner-global-gitconfig")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "0")
	env := gitAuthEnv("secret-token")
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "GIT_CONFIG_GLOBAL=/dev/null") {
		t.Fatalf("auth env does not disable global Git config")
	}
	if !strings.Contains(joined, "GIT_CONFIG_NOSYSTEM=1") {
		t.Fatalf("auth env does not disable system Git config")
	}
}

func TestGitAuthEnvPreservesAmbientConfigWithoutToken(t *testing.T) {
	if env := gitAuthEnv(""); env != nil {
		t.Fatalf("unauthenticated fetch env = %q, want nil", env)
	}
}
