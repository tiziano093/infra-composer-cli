package git

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// helperRunGit runs git in dir; tests skip when git is not available so
// CI hosts without the binary still pass.
func helperRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
}

func mustHaveGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	mustHaveGit(t)
	dir := t.TempDir()
	helperRunGit(t, dir, "init", "-q", "-b", "main")
	helperRunGit(t, dir, "config", "user.email", "test@example.com")
	helperRunGit(t, dir, "config", "user.name", "Test")
	helperRunGit(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

func TestRemoteURL_NormalizesSSH(t *testing.T) {
	dir := newRepo(t)
	helperRunGit(t, dir, "remote", "add", "origin", "git@github.com:acme/widgets.git")

	got, err := RemoteURL(context.Background(), dir, RemoteURLOptions{NormalizeToHTTPS: true})
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if want := "https://github.com/acme/widgets"; got != want {
		t.Fatalf("RemoteURL: got %q want %q", got, want)
	}
}

func TestRemoteURL_KeepsRawWhenAsked(t *testing.T) {
	dir := newRepo(t)
	helperRunGit(t, dir, "remote", "add", "origin", "git@gitlab.com:org/repo.git")
	got, err := RemoteURL(context.Background(), dir, RemoteURLOptions{})
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	// Default leaves the raw SSH URL alone; callers must opt in to
	// NormalizeToHTTPS when they need an HTTPS form.
	if got != "git@gitlab.com:org/repo.git" {
		t.Fatalf("expected raw URL, got %q", got)
	}
}

func TestRemoteURL_NoSuchRemote(t *testing.T) {
	dir := newRepo(t)
	_, err := RemoteURL(context.Background(), dir, RemoteURLOptions{Name: "upstream"})
	if !errors.Is(err, ErrNoRemote) {
		t.Fatalf("expected ErrNoRemote, got %v", err)
	}
}

func TestRemoteURL_NotARepo(t *testing.T) {
	mustHaveGit(t)
	_, err := RemoteURL(context.Background(), t.TempDir(), RemoteURLOptions{})
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("expected ErrNotARepository, got %v", err)
	}
}

func TestNormalizeRemoteURL(t *testing.T) {
	cases := map[string]string{
		"git@github.com:acme/widgets.git":       "https://github.com/acme/widgets",
		"git@github.com:acme/widgets":           "https://github.com/acme/widgets",
		"ssh://git@github.com/acme/widgets.git": "https://github.com/acme/widgets",
		"https://github.com/acme/widgets.git":   "https://github.com/acme/widgets",
		"https://github.com/acme/widgets":       "https://github.com/acme/widgets",
		"":                                      "",
	}
	for in, want := range cases {
		if got := NormalizeRemoteURL(in); got != want {
			t.Errorf("NormalizeRemoteURL(%q) = %q want %q", in, got, want)
		}
	}
}

func TestLatestSemverTag(t *testing.T) {
	dir := newRepo(t)
	for _, tag := range []string{"v0.1.0", "v0.2.0", "v0.10.0", "v0.10.0-rc1", "not-a-version"} {
		helperRunGit(t, dir, "tag", tag)
	}

	got, err := LatestSemverTag(context.Background(), dir, LatestSemverTagOptions{})
	if err != nil {
		t.Fatalf("LatestSemverTag: %v", err)
	}
	if got != "v0.10.0" {
		t.Fatalf("expected v0.10.0, got %q", got)
	}

	gotPre, err := LatestSemverTag(context.Background(), dir, LatestSemverTagOptions{IncludePrerelease: true})
	if err != nil {
		t.Fatalf("LatestSemverTag pre: %v", err)
	}
	// Stable still wins over its own pre-release.
	if gotPre != "v0.10.0" {
		t.Fatalf("expected v0.10.0, got %q", gotPre)
	}
}

func TestLatestSemverTag_NoMatchingTags(t *testing.T) {
	dir := newRepo(t)
	helperRunGit(t, dir, "tag", "release-candidate")
	got, err := LatestSemverTag(context.Background(), dir, LatestSemverTagOptions{})
	if err != nil {
		t.Fatalf("LatestSemverTag: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestLatestSemverTag_NotARepo(t *testing.T) {
	mustHaveGit(t)
	_, err := LatestSemverTag(context.Background(), t.TempDir(), LatestSemverTagOptions{})
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("expected ErrNotARepository, got %v", err)
	}
}

func TestCompareSemverTags(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.4", "v1.2.3", 1},
		{"v1.2.3", "v1.2.4", -1},
		{"v0.10.0", "v0.2.0", 1},
		{"v1.2.3", "v1.2.3-rc1", 1},
		{"v1.2.3-rc1", "v1.2.3", -1},
		{"1.0.0", "v1.0.0", 0},
	}
	for _, c := range cases {
		if got := compareSemverTags(c.a, c.b); got != c.want {
			t.Errorf("compare(%q, %q) = %d want %d", c.a, c.b, got, c.want)
		}
	}
}
