package git

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// semverTagPattern matches "v?MAJOR.MINOR.PATCH" with an optional
// pre-release segment, e.g. "v1.2.3", "1.2.3-rc1", "v0.10.0+build42".
var semverTagPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

// LatestSemverTagOptions tweaks the tag scan.
type LatestSemverTagOptions struct {
	// IncludePrerelease keeps tags with a "-suffix" segment in the
	// candidate set. Default false (production-friendly).
	IncludePrerelease bool
}

// LatestSemverTag returns the highest semver-ordered tag in the
// repository at dir, or an empty string when the repository has no
// matching tags. Errors only when git itself fails (missing binary,
// not a repository, etc).
func LatestSemverTag(ctx context.Context, dir string, opts LatestSemverTagOptions) (string, error) {
	out, err := runGit(ctx, dir, "tag", "--list")
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "not a git repository") {
			return "", ErrNotARepository
		}
		return "", fmt.Errorf("git tag --list: %v: %s", err, strings.TrimSpace(out))
	}
	tags := parseTags(out)
	candidates := filterSemverTags(tags, opts.IncludePrerelease)
	if len(candidates) == 0 {
		return "", nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return compareSemverTags(candidates[i], candidates[j]) > 0
	})
	return candidates[0], nil
}

// parseTags splits the multi-line output of "git tag --list" into
// trimmed entries, dropping blanks.
func parseTags(out string) []string {
	lines := strings.Split(out, "\n")
	res := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			res = append(res, l)
		}
	}
	return res
}

// filterSemverTags keeps only entries that match the semver pattern,
// optionally excluding pre-releases.
func filterSemverTags(tags []string, includePre bool) []string {
	res := make([]string, 0, len(tags))
	for _, t := range tags {
		if !semverTagPattern.MatchString(t) {
			continue
		}
		if !includePre && strings.Contains(t, "-") {
			continue
		}
		res = append(res, t)
	}
	return res
}

// compareSemverTags returns -1/0/1 for a vs b under semver-ish rules.
// Pre-release tags rank lower than their base version (so 1.2.3 > 1.2.3-rc1).
func compareSemverTags(a, b string) int {
	am := semverTagPattern.FindStringSubmatch(a)
	bm := semverTagPattern.FindStringSubmatch(b)
	if am == nil || bm == nil {
		return strings.Compare(a, b)
	}
	for i := 1; i <= 3; i++ {
		ai, _ := strconv.Atoi(am[i])
		bi, _ := strconv.Atoi(bm[i])
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	aPre := strings.Contains(a, "-")
	bPre := strings.Contains(b, "-")
	switch {
	case aPre && !bPre:
		return -1
	case !aPre && bPre:
		return 1
	case aPre && bPre:
		// Both pre-release with same numeric base: lexical fallback.
		return strings.Compare(a, b)
	}
	return 0
}
