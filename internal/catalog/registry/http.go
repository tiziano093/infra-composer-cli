package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

// DefaultRegistryBaseURL is the public Terraform Registry endpoint used
// by HTTPDiscovery when no override is supplied.
const DefaultRegistryBaseURL = "https://registry.terraform.io"

// HTTPDiscovery talks to the public Terraform Registry HTTP API to
// resolve provider versions and obtain the download metadata for the
// platform-specific provider binary.
//
// It deliberately does NOT implement registry.Client on its own: the
// registry HTTP API does not expose resource schemas. HTTPDiscovery is
// the first half of the live registry pipeline; the second half
// (downloading the binary and probing its gRPC schema) is layered on
// top by HTTPClient.
type HTTPDiscovery struct {
	baseURL string
	http    *http.Client
}

// HTTPDiscoveryOption configures a HTTPDiscovery instance.
type HTTPDiscoveryOption func(*HTTPDiscovery)

// WithHTTPClient overrides the http.Client used for outbound calls.
// Useful for tests (httptest.Server) and for callers that need custom
// timeouts or transport-level instrumentation.
func WithHTTPClient(c *http.Client) HTTPDiscoveryOption {
	return func(d *HTTPDiscovery) {
		if c != nil {
			d.http = c
		}
	}
}

// WithBaseURL overrides the registry base URL. The supplied value must
// not include a trailing slash.
func WithBaseURL(u string) HTTPDiscoveryOption {
	return func(d *HTTPDiscovery) {
		if u != "" {
			d.baseURL = strings.TrimRight(u, "/")
		}
	}
}

// NewHTTPDiscovery returns a HTTPDiscovery configured with the public
// Terraform Registry endpoint and a reasonable default timeout.
func NewHTTPDiscovery(opts ...HTTPDiscoveryOption) *HTTPDiscovery {
	d := &HTTPDiscovery{
		baseURL: DefaultRegistryBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ProviderDownload describes how to fetch a provider binary for a given
// operating system and architecture from the registry.
type ProviderDownload struct {
	OS              string
	Arch            string
	Filename        string
	DownloadURL     string
	ShasumsURL      string
	ShasumsSigURL   string
	Shasum          string
	Protocols       []string
}

// providerVersionsResponse mirrors the relevant portion of
// GET /v1/providers/{ns}/{name}/versions.
type providerVersionsResponse struct {
	ID       string `json:"id"`
	Versions []struct {
		Version   string `json:"version"`
		Protocols []string `json:"protocols"`
		Platforms []struct {
			OS   string `json:"os"`
			Arch string `json:"arch"`
		} `json:"platforms"`
	} `json:"versions"`
}

// providerDownloadResponse mirrors the relevant portion of
// GET /v1/providers/{ns}/{name}/{version}/download/{os}/{arch}.
type providerDownloadResponse struct {
	Protocols          []string `json:"protocols"`
	OS                 string   `json:"os"`
	Arch               string   `json:"arch"`
	Filename           string   `json:"filename"`
	DownloadURL        string   `json:"download_url"`
	ShasumsURL         string   `json:"shasums_url"`
	ShasumsSignatureURL string  `json:"shasums_signature_url"`
	Shasum             string   `json:"shasum"`
}

// ResolveProvider returns canonical ProviderInfo for the given address,
// pinning the version. If requestedVersion is empty or "latest", the
// most recent published version is returned.
//
// The Terraform Registry returns versions sorted lexicographically; we
// pick the one with the highest semver order to be deterministic.
func (d *HTTPDiscovery) ResolveProvider(ctx context.Context, address, requestedVersion string) (*ProviderInfo, error) {
	ns, name, err := splitAddress(address)
	if err != nil {
		return nil, err
	}
	versions, err := d.listVersions(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("%w: %s/%s has no published versions", ErrProviderNotFound, ns, name)
	}

	wanted := strings.TrimSpace(requestedVersion)
	if wanted == "" || strings.EqualFold(wanted, "latest") {
		picked := pickLatestVersion(versions)
		return &ProviderInfo{Namespace: ns, Name: name, Version: picked}, nil
	}
	for _, v := range versions {
		if v == wanted {
			return &ProviderInfo{Namespace: ns, Name: name, Version: v}, nil
		}
	}
	return nil, fmt.Errorf("%w: version %q not published for %s/%s", ErrProviderNotFound, wanted, ns, name)
}

// AvailableVersions returns the list of published versions for the
// given provider, in registry-declared order. Useful for the interactive
// command's version picker.
func (d *HTTPDiscovery) AvailableVersions(ctx context.Context, address string) ([]string, error) {
	ns, name, err := splitAddress(address)
	if err != nil {
		return nil, err
	}
	return d.listVersions(ctx, ns, name)
}

// DownloadInfo returns the platform-specific download metadata for the
// pinned provider version. When os/arch are empty the host runtime
// values are used.
func (d *HTTPDiscovery) DownloadInfo(ctx context.Context, p ProviderInfo, osName, arch string) (*ProviderDownload, error) {
	if p.Namespace == "" || p.Name == "" || p.Version == "" {
		return nil, fmt.Errorf("registry: ProviderInfo missing namespace/name/version: %+v", p)
	}
	if osName == "" {
		osName = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}

	endpoint := fmt.Sprintf("%s/v1/providers/%s/%s/%s/download/%s/%s",
		d.baseURL, url.PathEscape(p.Namespace), url.PathEscape(p.Name),
		url.PathEscape(p.Version), url.PathEscape(osName), url.PathEscape(arch))

	var dl providerDownloadResponse
	if err := d.getJSON(ctx, endpoint, &dl); err != nil {
		return nil, fmt.Errorf("download info %s/%s@%s (%s/%s): %w",
			p.Namespace, p.Name, p.Version, osName, arch, err)
	}
	if dl.DownloadURL == "" || dl.Shasum == "" {
		return nil, fmt.Errorf("registry: incomplete download info for %s/%s@%s (%s/%s)",
			p.Namespace, p.Name, p.Version, osName, arch)
	}
	return &ProviderDownload{
		OS:            dl.OS,
		Arch:          dl.Arch,
		Filename:      dl.Filename,
		DownloadURL:   dl.DownloadURL,
		ShasumsURL:    dl.ShasumsURL,
		ShasumsSigURL: dl.ShasumsSignatureURL,
		Shasum:        dl.Shasum,
		Protocols:     dl.Protocols,
	}, nil
}

// listVersions calls /v1/providers/{ns}/{name}/versions and returns the
// raw version strings. 404s are translated to ErrProviderNotFound.
func (d *HTTPDiscovery) listVersions(ctx context.Context, ns, name string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/v1/providers/%s/%s/versions",
		d.baseURL, url.PathEscape(ns), url.PathEscape(name))
	var resp providerVersionsResponse
	if err := d.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, fmt.Errorf("list versions %s/%s: %w", ns, name, err)
	}
	out := make([]string, 0, len(resp.Versions))
	for _, v := range resp.Versions {
		if v.Version != "" {
			out = append(out, v.Version)
		}
	}
	return out, nil
}

// getJSON performs a GET against url and decodes the response into v.
// A 404 is converted to ErrProviderNotFound; non-2xx responses become a
// formatted error including status code.
func (d *HTTPDiscovery) getJSON(ctx context.Context, endpoint string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "infra-composer-cli")

	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrProviderNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("registry returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	return nil
}

// PickLatestVersion is the exported alias for pickLatestVersion, used
// by callers (notably the interactive command) that need to resolve a
// "latest" choice against a list returned by AvailableVersions.
func PickLatestVersion(versions []string) string { return pickLatestVersion(versions) }

// pickLatestVersion returns the highest semver-ordered version from
// versions, falling back to the last entry when none parse cleanly.
// It tolerates pre-release suffixes by treating "1.2.3" > "1.2.3-rc1".
func pickLatestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	best := versions[0]
	for _, v := range versions[1:] {
		if compareSemver(v, best) > 0 {
			best = v
		}
	}
	return best
}

// compareSemver returns -1/0/1 ordering a vs b under loose semver.
// Non-numeric segments compare as strings; pre-release suffix demotes
// the version (so "1.2.3" > "1.2.3-rc1"). Errors return 0.
func compareSemver(a, b string) int {
	an, ap := splitSemver(a)
	bn, bp := splitSemver(b)
	for i := 0; i < len(an) || i < len(bn); i++ {
		var av, bv int
		if i < len(an) {
			av = an[i]
		}
		if i < len(bn) {
			bv = bn[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	// Equal numeric parts: a release version (no pre) is greater than a
	// pre-release version of the same numbers.
	if ap == "" && bp != "" {
		return 1
	}
	if bp == "" && ap != "" {
		return -1
	}
	if ap < bp {
		return -1
	}
	if ap > bp {
		return 1
	}
	return 0
}

// splitSemver splits "1.2.3-rc1+meta" into ([1,2,3], "rc1"). Build
// metadata after "+" is discarded per semver rules.
func splitSemver(v string) ([]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.Index(v, "+"); i >= 0 {
		v = v[:i]
	}
	core, pre := v, ""
	if i := strings.Index(v, "-"); i >= 0 {
		core, pre = v[:i], v[i+1:]
	}
	parts := strings.Split(core, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := atoiSafe(p)
		if err != nil {
			return nums, pre
		}
		nums = append(nums, n)
	}
	return nums, pre
}

func atoiSafe(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-numeric %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
