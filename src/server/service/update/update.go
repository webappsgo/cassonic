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

const githubReleasesURL = "https://api.github.com/repos/casjay/cassonic/releases/latest"

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
	httpClient     *http.Client
	logger         *log.Logger
}

// New creates a Checker that compares releases against currentVersion.
func New(currentVersion string, logger *log.Logger) *Checker {
	return &Checker{
		currentVersion: currentVersion,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		logger:         logger,
	}
}

// CheckLatest fetches the latest release from the GitHub Releases API.
// It extracts the version tag, publication timestamp, and the URL of the
// first attached asset (assumed to be the binary archive).
func (c *Checker) CheckLatest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
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
