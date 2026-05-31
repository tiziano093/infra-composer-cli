package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func searchSchema() *Schema {
	return &Schema{
		SchemaVersion:   SchemaVersion,
		Provider:        "hashicorp/aws",
		ProviderVersion: "5.42.0",
		Modules: []ModuleEntry{
			{Name: "aws_vpc", Type: ModuleTypeResource, Group: "network", Description: "Virtual Private Cloud"},
			{Name: "aws_vpc_peering", Type: ModuleTypeResource, Group: "network", Description: "VPC peering connection"},
			{Name: "aws_subnet", Type: ModuleTypeResource, Group: "network", Description: "Subnet inside a VPC"},
			{Name: "aws_instance", Type: ModuleTypeResource, Group: "compute", Description: "EC2 instance"},
			{Name: "aws_caller_identity", Type: ModuleTypeData, Group: "identity", Description: "Returns the current caller account"},
			{Name: "aws_s3_bucket", Type: ModuleTypeResource, Group: "storage", Description: "S3 bucket for object storage"},
		},
	}
}

func names(rs []SearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Module.Name
	}
	return out
}

func TestSearch_NoFiltersReturnsAllSortedByName(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{})
	require.Len(t, rs, 6)
	// All scores are zero, so order falls back to ascending name.
	assert.Equal(t, []string{
		"aws_caller_identity",
		"aws_instance",
		"aws_s3_bucket",
		"aws_subnet",
		"aws_vpc",
		"aws_vpc_peering",
	}, names(rs))
}

func TestSearch_NilSchemaReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, Search(nil, SearchOptions{Keywords: []string{"x"}}))
}

func TestSearch_KeywordSubstringRanksByScore(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"vpc"}})
	require.NotEmpty(t, rs)
	got := names(rs)
	// aws_vpc and aws_vpc_peering match by name; aws_subnet matches by
	// description ("inside a VPC"). Name matches must rank higher.
	assert.Contains(t, got, "aws_vpc")
	assert.Contains(t, got, "aws_vpc_peering")
	assert.Contains(t, got, "aws_subnet")
	assert.True(t, indexOf(got, "aws_vpc") < indexOf(got, "aws_subnet"))
	assert.True(t, indexOf(got, "aws_vpc_peering") < indexOf(got, "aws_subnet"))
}

func TestSearch_KeywordANDLogic(t *testing.T) {
	t.Parallel()
	// "network" matches the group; "peering" must additionally match
	// somewhere on the same module. Only aws_vpc_peering satisfies both.
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"network", "peering"}})
	require.Len(t, rs, 1)
	assert.Equal(t, "aws_vpc_peering", rs[0].Module.Name)
}

func TestSearch_KeywordIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"VPC"}})
	assert.NotEmpty(t, rs)
}

func TestSearch_KeywordNoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"nonexistent_thing_xyz"}})
	assert.Empty(t, rs)
}

func TestSearch_GroupFilter(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Group: "Network"}) // case-insensitive
	require.Len(t, rs, 3)
	for _, r := range rs {
		assert.Equal(t, "network", r.Module.Group)
	}
}

func TestSearch_TypeFilter(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Type: ModuleTypeData})
	require.Len(t, rs, 1)
	assert.Equal(t, "aws_caller_identity", rs[0].Module.Name)
}

func TestSearch_InvalidTypeFilterIsIgnored(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Type: ModuleType("module")})
	assert.Len(t, rs, 6)
}

func TestSearch_LimitCapsResults(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Limit: 2})
	assert.Len(t, rs, 2)
}

func TestSearch_NegativeLimitIsIgnored(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Limit: -1})
	assert.Len(t, rs, 6)
}

func TestSearch_FuzzyMatchCatchesSubsequence(t *testing.T) {
	t.Parallel()
	// "awvpc" is a subsequence of "aws_vpc" (a-w-v-p-c). Substring miss
	// must fall back to fuzzy and still produce a hit on the name.
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"awvpc"}})
	require.NotEmpty(t, rs)
	assert.Equal(t, "aws_vpc", rs[0].Module.Name)
}

func TestSearch_ExactNameOutranksSubstring(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"aws_vpc"}})
	require.NotEmpty(t, rs)
	// aws_vpc is an exact name match; aws_vpc_peering only contains it
	// as a substring. Exact must come first.
	assert.Equal(t, "aws_vpc", rs[0].Module.Name)
}

func TestSearch_EmptyKeywordEntriesAreSkipped(t *testing.T) {
	t.Parallel()
	rs := Search(searchSchema(), SearchOptions{Keywords: []string{"", "  ", "vpc"}})
	require.NotEmpty(t, rs)
	// Equivalent to searching just "vpc": both aws_vpc and aws_vpc_peering
	// match by name; the latter additionally has "vpc" in its description
	// so it ranks first. The test only cares that empty entries are not
	// treated as required terms (which would zero out every module).
	got := names(rs)
	assert.Contains(t, got, "aws_vpc")
	assert.Contains(t, got, "aws_vpc_peering")
}

func TestFuzzyMatch_EdgeCases(t *testing.T) {
	t.Parallel()
	assert.False(t, fuzzyMatch("", "abc"))
	assert.False(t, fuzzyMatch("abc", ""))
	assert.True(t, fuzzyMatch("abcde", "ace"))
	assert.False(t, fuzzyMatch("abcde", "aec"))
}

func indexOf(ss []string, target string) int {
	for i, s := range ss {
		if s == target {
			return i
		}
	}
	return -1
}
