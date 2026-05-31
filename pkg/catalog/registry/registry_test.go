package registry

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

func fixturesRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	// internal/catalog/registry/registry_test.go -> repo root.
	return filepath.Join(filepath.Dir(here), "..", "..", "..", "test", "fixtures", "registry")
}

func TestFakeClient_DiscoverProvider(t *testing.T) {
	c := NewFakeClient(fixturesRoot(t))
	got, err := c.DiscoverProvider(context.Background(), "hashicorp/aws")
	if err != nil {
		t.Fatalf("DiscoverProvider: %v", err)
	}
	want := ProviderInfo{Namespace: "hashicorp", Name: "aws", Version: "5.42.0"}
	if *got != want {
		t.Fatalf("got %+v want %+v", *got, want)
	}
	if got.Address() != "hashicorp/aws" {
		t.Fatalf("Address(): got %q", got.Address())
	}
}

func TestFakeClient_DiscoverProvider_BadAddress(t *testing.T) {
	c := NewFakeClient(fixturesRoot(t))
	cases := []string{"", "noslash", "/missingns", "ns/", "a/b/c"}
	for _, addr := range cases {
		if _, err := c.DiscoverProvider(context.Background(), addr); err == nil {
			t.Errorf("address %q: expected error", addr)
		}
	}
}

func TestFakeClient_DiscoverProvider_NotFound(t *testing.T) {
	c := NewFakeClient(fixturesRoot(t))
	_, err := c.DiscoverProvider(context.Background(), "hashicorp/missing")
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("want ErrProviderNotFound, got %v", err)
	}
}

func TestFakeClient_ListResources(t *testing.T) {
	c := NewFakeClient(fixturesRoot(t))
	p, err := c.DiscoverProvider(context.Background(), "hashicorp/aws")
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.ListResources(context.Background(), *p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("want 3 resources, got %d", len(res))
	}
	seen := map[string]Kind{}
	for _, r := range res {
		seen[r.Name] = r.Kind
	}
	if seen["aws_vpc"] != KindResource {
		t.Errorf("aws_vpc: want resource, got %q", seen["aws_vpc"])
	}
	if seen["aws_caller_identity"] != KindData {
		t.Errorf("aws_caller_identity: want data, got %q", seen["aws_caller_identity"])
	}
}

func TestFakeClient_GetResourceSchema(t *testing.T) {
	c := NewFakeClient(fixturesRoot(t))
	p := ProviderInfo{Namespace: "hashicorp", Name: "aws", Version: "5.42.0"}
	rs, err := c.GetResourceSchema(context.Background(), p, "aws_vpc", KindResource)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Name != "aws_vpc" || rs.Kind != KindResource {
		t.Fatalf("unexpected metadata: %+v", rs)
	}
	if len(rs.Inputs) != 3 || rs.Inputs[0].Name != "cidr_block" || !rs.Inputs[0].Required {
		t.Fatalf("inputs unexpected: %+v", rs.Inputs)
	}
	if len(rs.Outputs) != 2 || rs.Outputs[0].Name != "id" {
		t.Fatalf("outputs unexpected: %+v", rs.Outputs)
	}
}

func TestFakeClient_GetResourceSchema_NotFound(t *testing.T) {
	c := NewFakeClient(fixturesRoot(t))
	p := ProviderInfo{Namespace: "hashicorp", Name: "aws", Version: "5.42.0"}
	if _, err := c.GetResourceSchema(context.Background(), p, "aws_nope", KindResource); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("want ErrResourceNotFound, got %v", err)
	}
	// Wrong kind also surfaces as not-found (data source name vs resource).
	if _, err := c.GetResourceSchema(context.Background(), p, "aws_vpc", KindData); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("want ErrResourceNotFound for wrong kind, got %v", err)
	}
}

func TestKindIsValid(t *testing.T) {
	for k, want := range map[Kind]bool{
		KindResource: true,
		KindData:     true,
		Kind(""):     false,
		Kind("foo"):  false,
	} {
		if got := k.IsValid(); got != want {
			t.Errorf("Kind(%q).IsValid()=%v want %v", k, got, want)
		}
	}
}
