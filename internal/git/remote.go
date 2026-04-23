// Package git provides thin wrappers around the local git binary used
// by infra-composer to enrich generated Terraform module sources with
// the upstream remote URL and the latest semver tag.
//
// The package shells out instead of linking go-git to keep the binary
// small and rely on the user's existing git installation. All exported
// functions accept a working directory so callers can reuse them across
// repositories without changing process state.
package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// ErrNotARepository is returned when the supplied directory is not
// inside a git working tree.
var ErrNotARepository = errors.New("git: not a repository")

// ErrNoRemote signals that the repository has no remote with the
// requested name.
var ErrNoRemote = errors.New("git: no remote configured")

// RemoteURLOptions tweaks the behaviour of RemoteURL.
type RemoteURLOptions struct {
	// Name is the remote whose URL we want; defaults to "origin".
	Name string
	// NormalizeToHTTPS converts SSH URLs (git@host:owner/repo.git) to
	// HTTPS form (https://host/owner/repo) so they are usable as a
	// Terraform module "source" string. Defaults to true.
	NormalizeToHTTPS bool
}

// RemoteURL returns the configured URL for the named remote (default
// "origin"), optionally rewriting SSH-style URLs to HTTPS so they can
// be embedded in Terraform module source attributes.
func RemoteURL(ctx context.Context, dir string, opts RemoteURLOptions) (string, error) {
	if opts.Name == "" {
		opts.Name = "origin"
	}
	out, err := runGit(ctx, dir, "remote", "get-url", opts.Name)
	if err != nil {
		return "", classifyRemoteError(opts.Name, err, out)
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return "", fmt.Errorf("%w: %s", ErrNoRemote, opts.Name)
	}
	if !opts.NormalizeToHTTPS {
		return raw, nil
	}
	return NormalizeRemoteURL(raw), nil
}

// NormalizeRemoteURL rewrites the common SSH form (`git@host:owner/repo`
// or `ssh://git@host/owner/repo`) to its HTTPS counterpart. It also
// strips the trailing ".git" suffix that confuses some Terraform module
// resolvers. URLs that are already HTTP(S) are returned unchanged
// (apart from the ".git" trim).
func NormalizeRemoteURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	switch {
	case strings.HasPrefix(raw, "git@"):
		// git@github.com:owner/repo.git -> https://github.com/owner/repo
		rest := strings.TrimPrefix(raw, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if ok {
			return "https://" + host + "/" + strings.TrimSuffix(path, ".git")
		}
	case strings.HasPrefix(raw, "ssh://"):
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			path := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
			host := u.Hostname()
			return "https://" + host + "/" + path
		}
	}
	return strings.TrimSuffix(raw, ".git")
}

// runGit invokes the git binary in dir and returns the combined output.
// It is unexported because the package only uses a small set of curated
// commands (no general escape hatch for callers).
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// classifyRemoteError converts a raw exec error into a typed error so
// callers can distinguish "no repo" / "no such remote" from other
// failures (network, malformed config, etc).
func classifyRemoteError(name string, err error, out string) error {
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "not a git repository"):
		return ErrNotARepository
	case strings.Contains(low, "no such remote"):
		return fmt.Errorf("%w: %s", ErrNoRemote, name)
	}
	return fmt.Errorf("git remote get-url %s: %v: %s", name, err, strings.TrimSpace(out))
}
