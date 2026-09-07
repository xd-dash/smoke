// Package worktree materializes exact GitHub repository commits as detached,
// clean Git worktrees backed by a shared bare object database.
package worktree

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	shaPattern        = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

type Options struct {
	Repository         string
	SHA                string
	RoleRef            string
	Destination        string
	Token              string
	ObjectDatabaseRoot string
}

type Result struct {
	Repository     string `json:"repository"`
	SHA            string `json:"sha"`
	RoleRef        string `json:"role_ref,omitempty"`
	RoleRefSHA     string `json:"role_ref_sha,omitempty"`
	Worktree       string `json:"worktree"`
	ObjectDatabase string `json:"object_database"`
	GitDir         string `json:"git_dir"`
	Detached       bool   `json:"detached"`
	Clean          bool   `json:"clean"`
}

func Materialize(ctx context.Context, opts Options) (Result, error) {
	opts.Repository = strings.TrimSpace(opts.Repository)
	opts.SHA = strings.ToLower(strings.TrimSpace(opts.SHA))
	opts.RoleRef = strings.TrimSpace(opts.RoleRef)
	opts.Destination = strings.TrimSpace(opts.Destination)
	if !repositoryPattern.MatchString(opts.Repository) {
		return Result{}, fmt.Errorf("repository must be owner/name")
	}
	if !shaPattern.MatchString(opts.SHA) {
		return Result{}, fmt.Errorf("sha must be an exact 40-character commit")
	}
	if opts.RoleRef != "" {
		if strings.HasPrefix(opts.RoleRef, "-") {
			return Result{}, fmt.Errorf("role ref must not begin with '-'")
		}
		if strings.ContainsAny(opts.RoleRef, "\r\n") {
			return Result{}, fmt.Errorf("role ref must be one line")
		}
	}
	if opts.Destination == "" {
		return Result{}, fmt.Errorf("destination is required")
	}
	if _, err := os.Stat(opts.Destination); err == nil {
		return Result{}, fmt.Errorf("destination already exists: %s", opts.Destination)
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(opts.Destination), 0o700); err != nil {
		return Result{}, err
	}

	git, err := exec.LookPath("git")
	if err != nil {
		return Result{}, fmt.Errorf("ghxd worktree requires git: %w", err)
	}
	root := strings.TrimSpace(opts.ObjectDatabaseRoot)
	if root == "" {
		root = filepath.Join(os.TempDir(), "ghxd-git-components")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Result{}, err
	}

	repositoryKey := sha256.Sum256([]byte(opts.Repository))
	objectDatabase := filepath.Join(root, hex.EncodeToString(repositoryKey[:])+".git")
	origin := "https://github.com/" + opts.Repository + ".git"
	if _, err := os.Stat(objectDatabase); os.IsNotExist(err) {
		if err := run(ctx, "", git, nil, "init", "--bare", objectDatabase); err != nil {
			return Result{}, err
		}
		if err := run(ctx, "", git, nil, "--git-dir="+objectDatabase, "remote", "add", "origin", origin); err != nil {
			return Result{}, err
		}
	} else if err != nil {
		return Result{}, err
	} else {
		actualOrigin, err := output(ctx, "", git, nil, "--git-dir="+objectDatabase, "remote", "get-url", "origin")
		if err != nil {
			return Result{}, err
		}
		if actualOrigin != origin {
			return Result{}, fmt.Errorf("shared object database origin mismatch: expected %s got %s", origin, actualOrigin)
		}
		if err := run(ctx, "", git, nil, "--git-dir="+objectDatabase, "worktree", "prune"); err != nil {
			return Result{}, err
		}
	}

	authArgs := gitAuthArgs(opts.Token)
	fetchArgs := append(append([]string{}, authArgs...), "--git-dir="+objectDatabase, "fetch", "--no-tags", "origin", opts.SHA)
	if err := run(ctx, "", git, nil, fetchArgs...); err != nil {
		return Result{}, err
	}
	actualSHA, err := output(ctx, "", git, nil, "--git-dir="+objectDatabase, "rev-parse", opts.SHA+"^{commit}")
	if err != nil {
		return Result{}, err
	}
	if actualSHA != opts.SHA {
		return Result{}, fmt.Errorf("fetched commit %s does not match requested SHA %s", actualSHA, opts.SHA)
	}

	roleRefSHA := ""
	if opts.RoleRef != "" {
		refspec := "+refs/heads/" + opts.RoleRef + ":refs/remotes/origin/" + opts.RoleRef
		roleFetchArgs := append(append([]string{}, authArgs...), "--git-dir="+objectDatabase, "fetch", "--no-tags", "origin", refspec)
		if err := run(ctx, "", git, nil, roleFetchArgs...); err != nil {
			return Result{}, err
		}
		roleRefSHA, err = output(ctx, "", git, nil, "--git-dir="+objectDatabase, "rev-parse", "refs/remotes/origin/"+opts.RoleRef+"^{commit}")
		if err != nil {
			return Result{}, err
		}
		if err := run(ctx, "", git, nil, "--git-dir="+objectDatabase, "merge-base", "--is-ancestor", actualSHA, roleRefSHA); err != nil {
			return Result{}, fmt.Errorf("requested SHA %s is not reachable from role ref %s (%s): %w", actualSHA, opts.RoleRef, roleRefSHA, err)
		}
	}

	if err := run(ctx, "", git, nil, "--git-dir="+objectDatabase, "worktree", "add", "--detach", opts.Destination, actualSHA); err != nil {
		return Result{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = run(context.Background(), "", git, nil, "--git-dir="+objectDatabase, "worktree", "remove", "--force", opts.Destination)
		}
	}()

	headSHA, err := output(ctx, opts.Destination, git, nil, "rev-parse", "HEAD")
	if err != nil {
		return Result{}, err
	}
	if headSHA != actualSHA {
		return Result{}, fmt.Errorf("materialized HEAD %s does not match exact SHA %s", headSHA, actualSHA)
	}
	status, err := output(ctx, opts.Destination, git, nil, "status", "--porcelain")
	if err != nil {
		return Result{}, err
	}
	if status != "" {
		return Result{}, fmt.Errorf("materialized worktree is not clean")
	}
	if err := exec.CommandContext(ctx, git, "-C", opts.Destination, "symbolic-ref", "-q", "HEAD").Run(); err == nil {
		return Result{}, fmt.Errorf("materialized worktree is not detached")
	}

	worktreePath, err := filepath.EvalSymlinks(opts.Destination)
	if err != nil {
		worktreePath, err = filepath.Abs(opts.Destination)
		if err != nil {
			return Result{}, err
		}
	}
	objectDatabase, err = filepath.Abs(objectDatabase)
	if err != nil {
		return Result{}, err
	}
	gitDir, err := output(ctx, opts.Destination, git, nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return Result{}, err
	}
	cleanup = false
	return Result{
		Repository:     opts.Repository,
		SHA:            actualSHA,
		RoleRef:        opts.RoleRef,
		RoleRefSHA:     roleRefSHA,
		Worktree:       worktreePath,
		ObjectDatabase: objectDatabase,
		GitDir:         gitDir,
		Detached:       true,
		Clean:          true,
	}, nil
}

func gitAuthArgs(token string) []string {
	if token == "" {
		return nil
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	return []string{"-c", "http.https://github.com/.extraheader=AUTHORIZATION: basic " + encoded}
}

func run(ctx context.Context, dir, program string, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, program, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", filepath.Base(program), strings.Join(redactArgs(args), " "), err)
	}
	return nil
}

func output(ctx context.Context, dir, program string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	body, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", filepath.Base(program), strings.Join(redactArgs(args), " "), err)
	}
	return strings.TrimSpace(string(body)), nil
}

func redactArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i := range out {
		if strings.Contains(out[i], "AUTHORIZATION: basic ") {
			out[i] = "<redacted-auth-header>"
		}
	}
	return out
}
