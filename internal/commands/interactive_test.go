package commands

import (
	"strings"
	"testing"

	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
)

func TestValidateProviderAddress(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"hashicorp/random", false},
		{"hashicorp/aws", false},
		{" hashicorp/aws ", false},
		{"hashicorp", true},
		{"/aws", true},
		{"hashicorp/", true},
		{"a/b/c", true},
		{"", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			err := validateProviderAddress(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateProviderAddress(%q) err=%v wantErr=%v", tc.in, err, tc.wantErr)
			}
		})
	}
}

func TestValidateProviderAddress_NonString(t *testing.T) {
	if err := validateProviderAddress(42); err == nil {
		t.Fatal("expected error for non-string answer")
	}
}

func TestFormatAndParseSummary(t *testing.T) {
	s := registry.ResourceSummary{Name: "aws_vpc", Kind: registry.KindResource}
	label := formatSummary(s)
	if !strings.Contains(label, "aws_vpc") || !strings.Contains(label, "resource") {
		t.Fatalf("unexpected label: %q", label)
	}
	if got := parseSummaryName(label); got != "aws_vpc" {
		t.Fatalf("parseSummaryName: got %q want aws_vpc", got)
	}
	// Round-trip on something without the suffix returns the input.
	if got := parseSummaryName("aws_subnet"); got != "aws_subnet" {
		t.Fatalf("expected raw passthrough, got %q", got)
	}
}

func TestPresetProvidersIncludeOther(t *testing.T) {
	found := false
	for _, p := range presetProviders {
		if strings.HasPrefix(p, "Other") {
			found = true
		}
	}
	if !found {
		t.Fatal("preset list must include the freeform escape hatch")
	}
}

func TestBuildInteractiveClient_DefaultsToHomeCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c := buildInteractiveClient(&interactiveFlags{})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
