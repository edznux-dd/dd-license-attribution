// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package urlutil

import (
	"net/url"
	"regexp"
	"strings"
)

// GitURL represents a parsed git URL.
type GitURL struct {
	Valid    bool
	GitHub   bool
	Protocol string
	Host     string
	Owner    string
	Repo     string
	Branch   string
	PathRaw  string // path after owner/repo
}

var (
	// Match GitHub URLs in various formats
	githubHTTPRegex = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/?.]+?)(?:\.git)?(?:/(.*))?$`)
	// Match git@ SSH URLs
	githubSSHRegex = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/?.]+?)(?:\.git)?$`)
)

// ParseGitURL parses a git URL and extracts owner, repo, and other components.
func ParseGitURL(rawURL string) GitURL {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return GitURL{}
	}

	// Try SSH format first
	if m := githubSSHRegex.FindStringSubmatch(rawURL); m != nil {
		return GitURL{
			Valid:    true,
			GitHub:   true,
			Protocol: "ssh",
			Host:     "github.com",
			Owner:    m[1],
			Repo:     m[2],
		}
	}

	// Try HTTP(S) format
	if m := githubHTTPRegex.FindStringSubmatch(rawURL); m != nil {
		gu := GitURL{
			Valid:    true,
			GitHub:   true,
			Protocol: "https",
			Host:     "github.com",
			Owner:    m[1],
			Repo:     m[2],
		}
		if m[3] != "" {
			gu.PathRaw = "/" + m[3]
			// Extract branch from /tree/<branch> or /blob/<branch>
			if strings.HasPrefix(gu.PathRaw, "/tree/") {
				branchAndPath := strings.TrimPrefix(gu.PathRaw, "/tree/")
				gu.Branch = branchAndPath
			} else if strings.HasPrefix(gu.PathRaw, "/blob/") {
				branchAndPath := strings.TrimPrefix(gu.PathRaw, "/blob/")
				gu.Branch = branchAndPath
			}
		}
		return gu
	}

	// Generic URL parsing for non-GitHub URLs
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return GitURL{}
	}

	return GitURL{
		Valid:    true,
		GitHub:   false,
		Protocol: u.Scheme,
		Host:     u.Host,
	}
}

// IsGitHubURL checks if a URL points to GitHub.
func IsGitHubURL(rawURL string) bool {
	return ParseGitURL(rawURL).GitHub
}

// IsValidGitURL checks if a URL is a valid git URL.
func IsValidGitURL(rawURL string) bool {
	parsed := ParseGitURL(rawURL)
	return parsed.Valid
}

// CanonicalGitHubURL returns the canonical HTTPS URL for a GitHub repo.
func CanonicalGitHubURL(owner, repo string) string {
	return "https://github.com/" + owner + "/" + repo
}
