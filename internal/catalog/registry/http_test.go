package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubRegistry returns an httptest.Server emulating the relevant
// Terraform Registry endpoints for the given fixture map.
func stubRegistry(t *testing.T, versions map[string]any, downloads map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/providers/", func(w http.ResponseWriter, r *http.Request) {
		// Match either ".../versions" or ".../{version}/download/{os}/{arch}".
		path := r.URL.Path
		if v, ok := versions[path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
			return
		}
		if v, ok := downloads[path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(v)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPDiscovery_ResolveProvider_LatestSemver(t *testing.T) {
	srv := stubRegistry(t, map[string]any{
		"/v1/providers/hashicorp/random/versions": map[string]any{
			"id": "hashicorp/random",
			"versions": []map[string]any{
				{"version": "3.5.1"},
				{"version": "3.6.0-rc1"},
				{"version": "3.6.0"},
				{"version": "3.4.0"},
			},
		},
	}, nil)
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))

	got, err := d.ResolveProvider(context.Background(), "hashicorp/random", "")
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if got.Version != "3.6.0" {
		t.Fatalf("latest: got %q want 3.6.0", got.Version)
	}
}

func TestHTTPDiscovery_ResolveProvider_PinnedVersion(t *testing.T) {
	srv := stubRegistry(t, map[string]any{
		"/v1/providers/hashicorp/random/versions": map[string]any{
			"versions": []map[string]any{{"version": "3.5.1"}, {"version": "3.6.0"}},
		},
	}, nil)
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))

	got, err := d.ResolveProvider(context.Background(), "hashicorp/random", "3.5.1")
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if got.Version != "3.5.1" {
		t.Fatalf("pinned: got %q want 3.5.1", got.Version)
	}
}

func TestHTTPDiscovery_ResolveProvider_VersionMissing(t *testing.T) {
	srv := stubRegistry(t, map[string]any{
		"/v1/providers/hashicorp/random/versions": map[string]any{
			"versions": []map[string]any{{"version": "3.5.1"}},
		},
	}, nil)
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))

	_, err := d.ResolveProvider(context.Background(), "hashicorp/random", "9.9.9")
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("want ErrProviderNotFound, got %v", err)
	}
}

func TestHTTPDiscovery_ResolveProvider_ProviderMissing(t *testing.T) {
	srv := stubRegistry(t, nil, nil) // 404 for everything
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))

	_, err := d.ResolveProvider(context.Background(), "hashicorp/missing", "")
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("want ErrProviderNotFound, got %v", err)
	}
}

func TestHTTPDiscovery_ResolveProvider_BadAddress(t *testing.T) {
	d := NewHTTPDiscovery()
	for _, addr := range []string{"", "noslash", "/x", "x/"} {
		if _, err := d.ResolveProvider(context.Background(), addr, ""); err == nil {
			t.Errorf("address %q: expected error", addr)
		}
	}
}

func TestHTTPDiscovery_DownloadInfo_Success(t *testing.T) {
	srv := stubRegistry(t, nil, map[string]any{
		"/v1/providers/hashicorp/random/3.6.0/download/linux/amd64": map[string]any{
			"protocols":              []string{"5.0"},
			"os":                     "linux",
			"arch":                   "amd64",
			"filename":               "terraform-provider-random_3.6.0_linux_amd64.zip",
			"download_url":           "https://example.com/random_3.6.0_linux_amd64.zip",
			"shasums_url":            "https://example.com/random_3.6.0_SHA256SUMS",
			"shasums_signature_url":  "https://example.com/random_3.6.0_SHA256SUMS.sig",
			"shasum":                 "deadbeef",
		},
	})
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))
	p := ProviderInfo{Namespace: "hashicorp", Name: "random", Version: "3.6.0"}

	got, err := d.DownloadInfo(context.Background(), p, "linux", "amd64")
	if err != nil {
		t.Fatalf("DownloadInfo: %v", err)
	}
	if got.Filename != "terraform-provider-random_3.6.0_linux_amd64.zip" {
		t.Fatalf("filename: %q", got.Filename)
	}
	if got.Shasum != "deadbeef" || got.DownloadURL == "" || got.ShasumsURL == "" {
		t.Fatalf("incomplete: %+v", got)
	}
}

func TestHTTPDiscovery_DownloadInfo_MissingFields(t *testing.T) {
	srv := stubRegistry(t, nil, map[string]any{
		"/v1/providers/hashicorp/random/3.6.0/download/linux/amd64": map[string]any{
			"os":   "linux",
			"arch": "amd64",
			// download_url and shasum intentionally missing.
		},
	})
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))
	p := ProviderInfo{Namespace: "hashicorp", Name: "random", Version: "3.6.0"}

	if _, err := d.DownloadInfo(context.Background(), p, "linux", "amd64"); err == nil {
		t.Fatal("expected incomplete-info error")
	}
}

func TestHTTPDiscovery_DownloadInfo_PlatformMissing(t *testing.T) {
	srv := stubRegistry(t, nil, nil)
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))
	p := ProviderInfo{Namespace: "hashicorp", Name: "random", Version: "3.6.0"}

	if _, err := d.DownloadInfo(context.Background(), p, "plan9", "exotic"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("want ErrProviderNotFound, got %v", err)
	}
}

func TestHTTPDiscovery_AvailableVersions(t *testing.T) {
	srv := stubRegistry(t, map[string]any{
		"/v1/providers/hashicorp/random/versions": map[string]any{
			"versions": []map[string]any{{"version": "3.5.1"}, {"version": "3.6.0"}},
		},
	}, nil)
	d := NewHTTPDiscovery(WithBaseURL(srv.URL))

	vs, err := d.AvailableVersions(context.Background(), "hashicorp/random")
	if err != nil {
		t.Fatalf("AvailableVersions: %v", err)
	}
	if len(vs) != 2 || vs[0] != "3.5.1" || vs[1] != "3.6.0" {
		t.Fatalf("unexpected versions: %#v", vs)
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.99.99", 1},
		{"1.2.3", "1.2.3-rc1", 1},
		{"1.2.3-rc1", "1.2.3", -1},
		{"1.2.3-rc1", "1.2.3-rc2", -1},
		{"v1.2.3", "1.2.3", 0},
		{"1.2.3+meta", "1.2.3", 0},
	}
	for _, c := range cases {
		if got := compareSemver(c.a, c.b); got != c.want {
			t.Errorf("compareSemver(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
