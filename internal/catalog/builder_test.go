package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tiziano093/infra-composer-cli/internal/catalog/registry"
)

// stubClient is a hand-rolled registry.Client used to drive Builder
// scenarios without touching the disk-backed FakeClient.
type stubClient struct {
	provider *registry.ProviderInfo
	provErr  error
	listFn   func() ([]registry.ResourceSummary, error)
	schemaFn func(name string, kind registry.Kind) (*registry.ResourceSchema, error)
}

func (s *stubClient) DiscoverProvider(_ context.Context, address string) (*registry.ProviderInfo, error) {
	if s.provErr != nil {
		return nil, s.provErr
	}
	if s.provider != nil {
		return s.provider, nil
	}
	return &registry.ProviderInfo{Namespace: "ns", Name: "p", Version: "1.0.0"}, nil
}

func (s *stubClient) ListResources(_ context.Context, _ registry.ProviderInfo) ([]registry.ResourceSummary, error) {
	return s.listFn()
}

func (s *stubClient) GetResourceSchema(_ context.Context, _ registry.ProviderInfo, name string, kind registry.Kind) (*registry.ResourceSchema, error) {
	return s.schemaFn(name, kind)
}

func TestBuilder_Build_HappyPath(t *testing.T) {
	fixed := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	c := &stubClient{
		provider: &registry.ProviderInfo{Namespace: "hashicorp", Name: "aws", Version: "5.42.0"},
		listFn: func() ([]registry.ResourceSummary, error) {
			// Intentionally unsorted + mixed kinds to exercise sortModules.
			return []registry.ResourceSummary{
				{Name: "aws_subnet", Kind: registry.KindResource},
				{Name: "aws_caller_identity", Kind: registry.KindData},
				{Name: "aws_vpc", Kind: registry.KindResource},
			}, nil
		},
		schemaFn: func(name string, kind registry.Kind) (*registry.ResourceSchema, error) {
			return &registry.ResourceSchema{
				Name: name, Kind: kind, Group: "g", Description: "d",
				Inputs:  []registry.InputSpec{{Name: "a", Type: "string", Required: true}},
				Outputs: []registry.OutputSpec{{Name: "id"}},
			}, nil
		},
	}
	b := NewBuilder(c)
	s, err := b.Build(context.Background(), BuildOptions{
		Provider: "hashicorp/aws",
		Now:      func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if s.Provider != "hashicorp/aws" || s.ProviderVersion != "5.42.0" {
		t.Fatalf("provider metadata: %+v", s)
	}
	if !s.GeneratedAt.Equal(fixed) {
		t.Fatalf("GeneratedAt: got %v want %v", s.GeneratedAt, fixed)
	}
	if len(s.Modules) != 3 {
		t.Fatalf("modules: want 3, got %d", len(s.Modules))
	}
	// Resources first (alphabetical), then data.
	wantNames := []string{"aws_subnet", "aws_vpc", "aws_caller_identity"}
	for i, w := range wantNames {
		if s.Modules[i].Name != w {
			t.Errorf("modules[%d]: want %q got %q", i, w, s.Modules[i].Name)
		}
	}
	if s.Modules[2].Type != ModuleTypeData {
		t.Errorf("third module should be data, got %q", s.Modules[2].Type)
	}
}

func TestBuilder_Build_RequiresProvider(t *testing.T) {
	b := NewBuilder(&stubClient{})
	if _, err := b.Build(context.Background(), BuildOptions{}); err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestBuilder_Build_NilClient(t *testing.T) {
	b := NewBuilder(nil)
	if _, err := b.Build(context.Background(), BuildOptions{Provider: "ns/p"}); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestBuilder_Build_DiscoverError(t *testing.T) {
	want := errors.New("boom")
	b := NewBuilder(&stubClient{provErr: want})
	if _, err := b.Build(context.Background(), BuildOptions{Provider: "ns/p"}); !errors.Is(err, want) {
		t.Fatalf("want wrapped boom, got %v", err)
	}
}

func TestBuilder_Build_DuplicateModule(t *testing.T) {
	c := &stubClient{
		listFn: func() ([]registry.ResourceSummary, error) {
			return []registry.ResourceSummary{
				{Name: "x", Kind: registry.KindResource},
				{Name: "x", Kind: registry.KindResource},
			}, nil
		},
		schemaFn: func(name string, kind registry.Kind) (*registry.ResourceSchema, error) {
			return &registry.ResourceSchema{Name: name, Kind: kind}, nil
		},
	}
	b := NewBuilder(c)
	if _, err := b.Build(context.Background(), BuildOptions{Provider: "ns/p"}); err == nil {
		t.Fatal("expected duplicate module error")
	}
}

func TestBuilder_Build_InvalidKind(t *testing.T) {
	c := &stubClient{
		listFn: func() ([]registry.ResourceSummary, error) {
			return []registry.ResourceSummary{{Name: "x", Kind: registry.Kind("bogus")}}, nil
		},
	}
	b := NewBuilder(c)
	if _, err := b.Build(context.Background(), BuildOptions{Provider: "ns/p"}); err == nil {
		t.Fatal("expected invalid kind error")
	}
}

func TestBuilder_Build_FetchSchemaError(t *testing.T) {
	c := &stubClient{
		listFn: func() ([]registry.ResourceSummary, error) {
			return []registry.ResourceSummary{{Name: "x", Kind: registry.KindResource}}, nil
		},
		schemaFn: func(_ string, _ registry.Kind) (*registry.ResourceSchema, error) {
			return nil, registry.ErrResourceNotFound
		},
	}
	b := NewBuilder(c)
	if _, err := b.Build(context.Background(), BuildOptions{Provider: "ns/p"}); !errors.Is(err, registry.ErrResourceNotFound) {
		t.Fatalf("want ErrResourceNotFound chain, got %v", err)
	}
}

func TestBuilder_Build_FromFakeFixture(t *testing.T) {
	c := registry.NewFakeClient(repoFixturesDir(t, "registry"))
	b := NewBuilder(c)
	s, err := b.Build(context.Background(), BuildOptions{
		Provider: "hashicorp/aws",
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(s.Modules) != 3 {
		t.Fatalf("want 3 modules, got %d", len(s.Modules))
	}
}

func TestBuilder_Build_IncludeExcludeFilters(t *testing.T) {
	t.Helper()
	// Reuse the existing fake provider fixture under
	// test/fixtures/registry/hashicorp/aws which exposes 3 modules:
	// aws_vpc, aws_subnet (resources) and aws_caller_identity (data).
	root := repoFixturesDir(t, "registry")
	client := registry.NewFakeClient(root)
	b := NewBuilder(client)

	// Include only modules starting with "aws_v*" -> aws_vpc.
	s, err := b.Build(context.Background(), BuildOptions{
		Provider: "hashicorp/aws",
		Include:  []string{"aws_v*"},
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("build with include: %v", err)
	}
	if len(s.Modules) != 1 || s.Modules[0].Name != "aws_vpc" {
		t.Fatalf("include filter: got %+v", moduleNames(s.Modules))
	}

	// Exclude data sources by pattern -> aws_caller_identity gone.
	s, err = b.Build(context.Background(), BuildOptions{
		Provider: "hashicorp/aws",
		Exclude:  []string{"aws_caller_*"},
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err != nil {
		t.Fatalf("build with exclude: %v", err)
	}
	for _, m := range s.Modules {
		if m.Name == "aws_caller_identity" {
			t.Fatalf("exclude filter let aws_caller_identity through: %+v", moduleNames(s.Modules))
		}
	}
}

func moduleNames(ms []ModuleEntry) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}
