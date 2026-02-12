// SPDX-License-Identifier: Apache-2.0
package urlutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitURL_GitHubHTTPS(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		owner    string
		repo     string
		branch   string
		github   bool
	}{
		{
			name:   "simple repo",
			url:    "https://github.com/DataDog/dd-license-attribution",
			owner:  "DataDog",
			repo:   "dd-license-attribution",
			github: true,
		},
		{
			name:   "with .git suffix",
			url:    "https://github.com/DataDog/dd-license-attribution.git",
			owner:  "DataDog",
			repo:   "dd-license-attribution",
			github: true,
		},
		{
			name:   "with tree path",
			url:    "https://github.com/DataDog/dd-license-attribution/tree/main/src",
			owner:  "DataDog",
			repo:   "dd-license-attribution",
			branch: "main/src",
			github: true,
		},
		{
			name:   "with blob path",
			url:    "https://github.com/DataDog/dd-license-attribution/blob/main/README.md",
			owner:  "DataDog",
			repo:   "dd-license-attribution",
			branch: "main/README.md",
			github: true,
		},
		{
			name:   "http scheme",
			url:    "http://github.com/DataDog/dd-license-attribution",
			owner:  "DataDog",
			repo:   "dd-license-attribution",
			github: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseGitURL(tt.url)
			assert.True(t, parsed.Valid)
			assert.Equal(t, tt.github, parsed.GitHub)
			assert.Equal(t, tt.owner, parsed.Owner)
			assert.Equal(t, tt.repo, parsed.Repo)
			if tt.branch != "" {
				assert.Equal(t, tt.branch, parsed.Branch)
			}
		})
	}
}

func TestParseGitURL_SSH(t *testing.T) {
	parsed := ParseGitURL("git@github.com:DataDog/dd-license-attribution.git")
	assert.True(t, parsed.Valid)
	assert.True(t, parsed.GitHub)
	assert.Equal(t, "DataDog", parsed.Owner)
	assert.Equal(t, "dd-license-attribution", parsed.Repo)
}

func TestParseGitURL_NonGitHub(t *testing.T) {
	parsed := ParseGitURL("https://gitlab.com/some/repo")
	assert.True(t, parsed.Valid)
	assert.False(t, parsed.GitHub)
}

func TestParseGitURL_Invalid(t *testing.T) {
	parsed := ParseGitURL("")
	assert.False(t, parsed.Valid)

	parsed = ParseGitURL("not-a-url")
	assert.False(t, parsed.Valid)
}

func TestIsGitHubURL(t *testing.T) {
	assert.True(t, IsGitHubURL("https://github.com/foo/bar"))
	assert.False(t, IsGitHubURL("https://gitlab.com/foo/bar"))
}

func TestIsValidGitURL(t *testing.T) {
	assert.True(t, IsValidGitURL("https://github.com/foo/bar"))
	assert.True(t, IsValidGitURL("https://gitlab.com/foo/bar"))
	assert.False(t, IsValidGitURL(""))
}

func TestCanonicalGitHubURL(t *testing.T) {
	assert.Equal(t, "https://github.com/DataDog/dd-license-attribution",
		CanonicalGitHubURL("DataDog", "dd-license-attribution"))
}
