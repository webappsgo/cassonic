// Package update checks for new cassonic releases from the GitHub API.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	githubReleasesLatestURL = "https://api.github.com/repos/casjay/cassonic/releases/latest"
	githubReleasesListURL   = "https://api.github.com/repos/casjay/cassonic/releases?per_page=20"
)

// Release describes a published cassonic release.
type Release struct {
	Version     string
	PublishedAt time.Time
	DownloadURL string
	Checksum    string
}

// Checker fetches and compares release information from GitHub.
type Checker struct {
	currentVersion string
	branch         string
	httpClient     *http.Client
	logger         *log.Logger
}

// New creates a Checker that compares releases against currentVersion.
// The default branch is "stable" (latest tagged release).
func New(currentVersion string, logger *log.Logger) *Checker {
	return &Checker{
		currentVersion: currentVersion,
		branch:         "stable",
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		logger:         logger,
	}
}

// WithBranch sets the release channel used when fetching updates.
// Valid values: "stable" (default), "beta", "daily".
func (c *Checker) WithBranch(branch string) *Checker {
	switch branch {
	case "stable", "beta", "daily":
		c.branch = branch
	default:
		c.logger.Printf("update: unknown branch %q, using stable", branch)
		c.branch = "stable"
	}
	return c
}

// Branch returns the currently configured release channel.
func (c *Checker) Branch() string {
	return c.branch
}

// CheckLatest fetches the latest release from the GitHub Releases API for the
// configured branch. Stable → /releases/latest; beta/daily → /releases list
// filtered by version suffix pattern.
func (c *Checker) CheckLatest(ctx context.Context) (*Release, error) {
	if c.branch == "stable" {
		return c.fetchLatestStable(ctx)
	}
	return c.fetchLatestFiltered(ctx, c.branch)
}

// fetchLatestStable calls the /releases/latest endpoint (stable channel).
func (c *Checker) fetchLatestStable(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesLatestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cassonic/"+c.currentVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch releases: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("update: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: GitHub API returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			BrowserDownloadURL string `json:"browser_download_url"`
			Name               string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("update: parse response: %w", err)
	}

	rel := &Release{
		Version: strings.TrimPrefix(payload.TagName, "v"),
	}

	if payload.PublishedAt != "" {
		t, parseErr := time.Parse(time.RFC3339, payload.PublishedAt)
		if parseErr == nil {
			rel.PublishedAt = t
		}
	}

	for _, asset := range payload.Assets {
		if asset.BrowserDownloadURL != "" {
			rel.DownloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return rel, nil
}

// fetchLatestFiltered fetches all recent releases and returns the newest one
// whose tag matches the requested channel pattern:
//   - "beta"  → tag contains "-beta"
//   - "daily" → tag matches YYYYMMDDHHMMSS (14-digit numeric prefix)
func (c *Checker) fetchLatestFiltered(ctx context.Context, channel string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cassonic/"+c.currentVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: fetch releases: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("update: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: GitHub API returned HTTP %d", resp.StatusCode)
	}

	var releases []struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Prerelease  bool   `json:"prerelease"`
		Assets      []struct {
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("update: parse response: %w", err)
	}

	for _, r := range releases {
		tag := strings.TrimPrefix(r.TagName, "v")
		match := false
		switch channel {
		case "beta":
			match = strings.Contains(tag, "-beta")
		case "daily":
			// Daily tags are 14-digit numeric timestamps (YYYYMMDDHHMMSS).
			if len(tag) >= 14 {
				_, parseErr := strconv.ParseInt(tag[:14], 10, 64)
				match = parseErr == nil
			}
		}
		if !match {
			continue
		}

		rel := &Release{Version: tag}
		if r.PublishedAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, r.PublishedAt); parseErr == nil {
				rel.PublishedAt = t
			}
		}
		for _, a := range r.Assets {
			if a.BrowserDownloadURL != "" {
				rel.DownloadURL = a.BrowserDownloadURL
				break
			}
		}
		return rel, nil
	}

	return nil, fmt.Errorf("update: no %s releases found", channel)
}

// IsNewer reports whether release.Version is strictly greater than the checker's
// current version using semantic version comparison.
// Non-semver strings (e.g. "dev") are treated as the lowest possible version.
func (c *Checker) IsNewer(r *Release) bool {
	return semverLess(c.currentVersion, r.Version)
}

// semverLess returns true when version a is strictly less than b.
// Version strings are split on "." and compared component by component as integers.
// Non-numeric components fall back to lexicographic comparison.
func semverLess(a, b string) bool {
	partsA := versionParts(a)
	partsB := versionParts(b)

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var pa, pb int
		if i < len(partsA) {
			pa = partsA[i]
		}
		if i < len(partsB) {
			pb = partsB[i]
		}
		if pa < pb {
			return true
		}
		if pa > pb {
			return false
		}
	}

	return false
}

// versionParts splits a version string on "." and returns each component as an integer.
// Non-numeric or empty components are treated as 0.
func versionParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	segments := strings.Split(v, ".")
	parts := make([]int, 0, len(segments))
	for _, seg := range segments {
		n, err := strconv.Atoi(seg)
		if err != nil {
			n = 0
		}
		parts = append(parts, n)
	}
	return parts
}
