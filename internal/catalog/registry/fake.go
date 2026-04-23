package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FakeClient is a Client implementation backed by JSON fixture files on
// disk. Each provider lives in `<root>/<namespace>/<name>/provider.json`
// using the wire format below. It is intended for unit and integration
// tests as well as the default `catalog build` experience until the real
// Terraform Registry HTTP client lands.
//
// Wire format (provider.json):
//
//	{
//	  "namespace": "hashicorp",
//	  "name": "aws",
//	  "version": "5.42.0",
//	  "resources": [
//	    {
//	      "name": "aws_vpc",
//	      "kind": "resource",
//	      "group": "network",
//	      "source": "https://github.com/hashicorp/terraform-provider-aws",
//	      "description": "Virtual Private Cloud",
//	      "inputs":  [{"name": "cidr_block", "type": "string", "required": true}],
//	      "outputs": [{"name": "id"}]
//	    }
//	  ]
//	}
type FakeClient struct {
	root string

	mu    sync.Mutex
	cache map[string]*fakeProvider
}

// NewFakeClient returns a FakeClient that resolves provider fixtures
// relative to root. The directory does not have to exist yet; lookups
// will surface ErrProviderNotFound when the file is missing.
func NewFakeClient(root string) *FakeClient {
	return &FakeClient{root: root, cache: map[string]*fakeProvider{}}
}

// fakeProvider is the on-disk JSON shape consumed by FakeClient.
type fakeProvider struct {
	Namespace string             `json:"namespace"`
	Name      string             `json:"name"`
	Version   string             `json:"version"`
	Resources []fakeResourceJSON `json:"resources"`
}

type fakeResourceJSON struct {
	Name        string           `json:"name"`
	Kind        Kind             `json:"kind"`
	Group       string           `json:"group,omitempty"`
	Source      string           `json:"source,omitempty"`
	Description string           `json:"description,omitempty"`
	Inputs      []fakeInputJSON  `json:"inputs,omitempty"`
	Outputs     []fakeOutputJSON `json:"outputs,omitempty"`
}

type fakeInputJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

type fakeOutputJSON struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

// DiscoverProvider implements Client.
func (c *FakeClient) DiscoverProvider(_ context.Context, address string) (*ProviderInfo, error) {
	ns, name, err := splitAddress(address)
	if err != nil {
		return nil, err
	}
	prov, err := c.load(ns, name)
	if err != nil {
		return nil, err
	}
	return &ProviderInfo{Namespace: prov.Namespace, Name: prov.Name, Version: prov.Version}, nil
}

// ListResources implements Client.
func (c *FakeClient) ListResources(_ context.Context, p ProviderInfo) ([]ResourceSummary, error) {
	prov, err := c.load(p.Namespace, p.Name)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceSummary, 0, len(prov.Resources))
	for _, r := range prov.Resources {
		out = append(out, ResourceSummary{Name: r.Name, Kind: r.Kind})
	}
	return out, nil
}

// GetResourceSchema implements Client.
func (c *FakeClient) GetResourceSchema(_ context.Context, p ProviderInfo, name string, kind Kind) (*ResourceSchema, error) {
	prov, err := c.load(p.Namespace, p.Name)
	if err != nil {
		return nil, err
	}
	for _, r := range prov.Resources {
		if r.Name == name && r.Kind == kind {
			return convertResource(r), nil
		}
	}
	return nil, fmt.Errorf("%w: %s/%s %s %q", ErrResourceNotFound, p.Namespace, p.Name, kind, name)
}

// load resolves and caches a provider fixture. Cache keys are address
// strings; the cache is populated lazily so unrelated tests do not pay
// for fixtures they don't touch.
func (c *FakeClient) load(namespace, name string) (*fakeProvider, error) {
	key := namespace + "/" + name
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.cache[key]; ok {
		return cached, nil
	}
	path := filepath.Join(c.root, namespace, name, "provider.json")
	f, err := os.Open(path)
	if err != nil {
		if errorsIsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, key)
		}
		return nil, fmt.Errorf("registry: open fixture %s: %w", path, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var prov fakeProvider
	if err := dec.Decode(&prov); err != nil {
		return nil, fmt.Errorf("registry: decode fixture %s: %w", path, err)
	}
	if prov.Namespace == "" || prov.Name == "" || prov.Version == "" {
		return nil, fmt.Errorf("registry: fixture %s missing namespace/name/version", path)
	}
	if prov.Namespace != namespace || prov.Name != name {
		return nil, fmt.Errorf("registry: fixture %s declares %s/%s but lives under %s/%s",
			path, prov.Namespace, prov.Name, namespace, name)
	}
	c.cache[key] = &prov
	return &prov, nil
}

// splitAddress validates and splits a "<namespace>/<name>" address.
func splitAddress(address string) (string, string, error) {
	parts := strings.SplitN(address, "/", 3)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("registry: invalid provider address %q (expected \"<namespace>/<name>\")", address)
	}
	return parts[0], parts[1], nil
}

func convertResource(r fakeResourceJSON) *ResourceSchema {
	rs := &ResourceSchema{
		Name:        r.Name,
		Kind:        r.Kind,
		Group:       r.Group,
		Source:      r.Source,
		Description: r.Description,
	}
	for _, i := range r.Inputs {
		rs.Inputs = append(rs.Inputs, InputSpec(i))
	}
	for _, o := range r.Outputs {
		rs.Outputs = append(rs.Outputs, OutputSpec(o))
	}
	return rs
}

// errorsIsNotExist is a tiny shim so the test file does not need to
// import os just to inspect this single sentinel.
func errorsIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || isFsErrNotExist(err))
}

// isFsErrNotExist handles fs.ErrNotExist for fs-based wrappers (kept
// separate to make the intent explicit to readers).
func isFsErrNotExist(err error) bool {
	if pe, ok := err.(*fs.PathError); ok {
		return os.IsNotExist(pe.Err)
	}
	return false
}
