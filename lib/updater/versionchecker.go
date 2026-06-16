package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ghAsset / ghRelease mirror the subset of the GitHub releases API we consume.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt string    `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []ghAsset `json:"assets"`
}

func (r ghRelease) toReleaseInfo() *ReleaseInfo {
	return &ReleaseInfo{
		Version:     strings.TrimPrefix(r.TagName, "v"),
		Tag:         r.TagName,
		Body:        r.Body,
		PublishedAt: r.PublishedAt,
		Prerelease:  r.Prerelease,
		HTMLURL:     r.HTMLURL,
	}
}

// CheckKind is the discriminator for a CheckResult.
type CheckKind string

const (
	CheckUpdated     CheckKind = "updated"
	CheckNotModified CheckKind = "notmodified"
	CheckPrerelease  CheckKind = "skipped-prerelease"
	CheckRateLimited CheckKind = "ratelimited"
	CheckError       CheckKind = "error"
)

// CheckResult is the outcome of polling for the latest release.
type CheckResult struct {
	Kind    CheckKind
	Release *ReleaseInfo
	ETag    string
	Status  int
}

// VersionChecker polls the GitHub releases API with ETag-based caching.
type VersionChecker struct {
	HTTPClient *http.Client
	Repo       string // e.g. "ether/etherpad-go"
	apiBase    string // overridable in tests
}

// NewVersionChecker builds a checker for the given repo.
func NewVersionChecker(repo string) *VersionChecker {
	return &VersionChecker{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Repo:       repo,
		apiBase:    "https://api.github.com",
	}
}

// CheckLatest fetches the latest (non-prerelease) release, sending the previous
// ETag so an unchanged release returns notmodified cheaply.
func (v *VersionChecker) CheckLatest(prevETag string) CheckResult {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", v.apiBase, v.Repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return CheckResult{Kind: CheckError}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if prevETag != "" {
		req.Header.Set("If-None-Match", prevETag)
	}
	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return CheckResult{Kind: CheckError}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return CheckResult{Kind: CheckNotModified, ETag: prevETag, Status: resp.StatusCode}
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return CheckResult{Kind: CheckRateLimited, Status: resp.StatusCode}
	case resp.StatusCode != http.StatusOK:
		return CheckResult{Kind: CheckError, Status: resp.StatusCode}
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return CheckResult{Kind: CheckError, Status: resp.StatusCode}
	}
	if rel.Prerelease || rel.Draft {
		return CheckResult{Kind: CheckPrerelease, ETag: resp.Header.Get("ETag"), Status: resp.StatusCode}
	}
	if !IsValidTag(rel.TagName) {
		return CheckResult{Kind: CheckError, Status: resp.StatusCode}
	}
	return CheckResult{Kind: CheckUpdated, Release: rel.toReleaseInfo(), ETag: resp.Header.Get("ETag"), Status: resp.StatusCode}
}

// fetchReleaseByTag returns the full release (including assets) for one tag.
func (v *VersionChecker) fetchReleaseByTag(tag string) (*ghRelease, error) {
	if !IsValidTag(tag) {
		return nil, fmt.Errorf("invalid tag %q", tag)
	}
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", v.apiBase, v.Repo, tag)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github returned %d fetching release %s: %s", resp.StatusCode, tag, string(body))
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}
