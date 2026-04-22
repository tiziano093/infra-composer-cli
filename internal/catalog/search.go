package catalog

import (
	"sort"
	"strings"
)

// SearchOptions parameterises Search. Zero value matches everything in
// the catalog with no ordering preference and no limit.
type SearchOptions struct {
	// Keywords are matched with AND logic: every keyword must match at
	// least one searchable field of a module for that module to be
	// returned. Keywords are case-insensitive. An empty slice means
	// "no keyword filter".
	Keywords []string

	// Group, if non-empty, restricts results to modules whose Group
	// equals this value (case-insensitive).
	Group string

	// Type, if non-empty, restricts results to a single ModuleType.
	// Invalid values are treated as "no filter".
	Type ModuleType

	// Limit caps the number of returned results. Zero or negative
	// means "no limit".
	Limit int
}

// SearchResult is a scored module match returned by Search. Higher
// Score means better relevance; ties are broken by module name to keep
// output deterministic across runs.
type SearchResult struct {
	Module ModuleEntry
	Score  int
}

// Field weights used to compute SearchResult.Score. Tuned so that a
// keyword in the name dominates the same keyword in the description.
const (
	scoreNameExact     = 100
	scoreNameSubstring = 30
	scoreNameFuzzy     = 10
	scoreGroup         = 15
	scoreDescription   = 5
)

// Search filters and ranks modules in s according to opts. The result
// is sorted by descending Score, then by ascending module name. A nil
// or empty schema yields a nil slice.
func Search(s *Schema, opts SearchOptions) []SearchResult {
	if s == nil || len(s.Modules) == 0 {
		return nil
	}
	keywords := normaliseKeywords(opts.Keywords)
	group := strings.ToLower(strings.TrimSpace(opts.Group))
	typeFilter := opts.Type
	if !typeFilter.IsValid() {
		typeFilter = ""
	}

	results := make([]SearchResult, 0, len(s.Modules))
	for _, m := range s.Modules {
		if typeFilter != "" && m.Type != typeFilter {
			continue
		}
		if group != "" && strings.ToLower(m.Group) != group {
			continue
		}
		score, ok := scoreModule(m, keywords)
		if !ok {
			continue
		}
		results = append(results, SearchResult{Module: m, Score: score})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Module.Name < results[j].Module.Name
	})

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results
}

// normaliseKeywords lowercases, trims and drops empty keywords. The
// returned slice is nil when the input contains no usable terms, which
// signals "no keyword filter" to scoreModule.
func normaliseKeywords(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, k := range in {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// scoreModule returns the aggregate score for m against keywords plus
// a boolean reporting whether the module satisfies the AND-match rule.
// When keywords is empty every module matches with score 0.
func scoreModule(m ModuleEntry, keywords []string) (int, bool) {
	if len(keywords) == 0 {
		return 0, true
	}
	name := strings.ToLower(m.Name)
	desc := strings.ToLower(m.Description)
	group := strings.ToLower(m.Group)

	total := 0
	for _, kw := range keywords {
		hit := 0
		switch {
		case name == kw:
			hit += scoreNameExact
		case strings.Contains(name, kw):
			hit += scoreNameSubstring
		case fuzzyMatch(name, kw):
			hit += scoreNameFuzzy
		}
		if group != "" && strings.Contains(group, kw) {
			hit += scoreGroup
		}
		if desc != "" && strings.Contains(desc, kw) {
			hit += scoreDescription
		}
		if hit == 0 {
			return 0, false
		}
		total += hit
	}
	return total, true
}

// fuzzyMatch reports whether all bytes of needle appear in haystack in
// order, allowing arbitrary characters between them. Both inputs must
// already be lowercased. Empty needle never matches (caller guarantees
// non-empty keywords).
//
// This is a deliberately simple subsequence match: cheap, predictable
// and good enough to catch missing letters or minor typos
// ("vp" -> "aws_vpc"). Heavier approaches (Levenshtein, n-grams) can
// replace it later behind the same function signature.
func fuzzyMatch(haystack, needle string) bool {
	if needle == "" || haystack == "" {
		return false
	}
	i := 0
	for j := 0; j < len(haystack) && i < len(needle); j++ {
		if haystack[j] == needle[i] {
			i++
		}
	}
	return i == len(needle)
}
