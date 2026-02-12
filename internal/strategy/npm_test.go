// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNpmTestDeps(t *testing.T) (context.Context, *artifact.SourceCodeManager, *testutil.MockFileSystem, *testutil.MockProcessRunner, *testutil.MockHTTPFetcher) {
	t.Helper()
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"org/test-npm-repo": {
				HTMLURL:       "https://github.com/org/test-npm-repo",
				URL:           "https://api.github.com/repos/org/test-npm-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "org"},
			},
		},
		RepoStatuses: map[string]int{"org/test-npm-repo": 200},
	}

	fs := &testutil.MockFileSystem{
		Dirs:  map[string]bool{"/cache": true},
		Files: map[string][]byte{},
	}
	proc := &testutil.MockProcessRunner{
		Outputs: map[string]string{
			"git ls-remote --symref https://github.com/org/test-npm-repo HEAD": "ref: refs/heads/main\tHEAD\n",
		},
	}
	http := &testutil.MockHTTPFetcher{
		Responses: map[string]*port.HTTPResponse{},
	}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, err := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)
	require.NoError(t, err)

	return context.Background(), scm, fs, proc, http
}

func TestNpmStrategy_ExtractLicenseFromPkgData(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{"nil", nil, nil},
		{"string", "MIT", []string{"MIT"}},
		{"empty string", "", nil},
		{"object with type", map[string]interface{}{"type": "Apache-2.0"}, []string{"Apache-2.0"}},
		{"object without type", map[string]interface{}{"url": "http://..."}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLicenseFromPkgData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNpmStrategy_ExtractCopyrightFromAuthor(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{"nil", nil, nil},
		{"string", "John Doe", []string{"John Doe"}},
		{"empty string", "", nil},
		{"object with name", map[string]interface{}{"name": "Jane Doe"}, []string{"Jane Doe"}},
		{"object without name", map[string]interface{}{"email": "j@example.com"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCopyrightFromAuthor(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNpmStrategy_DetermineOrigin(t *testing.T) {
	ctx, scm, fs, proc, http := newNpmTestDeps(t)

	s := NewNpmStrategy(ctx, "https://github.com/org/test-npm-repo", scm, metadata.ScopeAll, nil, fs, proc, http)

	t.Run("nil pkgData returns npm: prefix", func(t *testing.T) {
		result := s.determineOrigin(nil, "lodash")
		assert.Equal(t, "npm:lodash", result)
	})

	t.Run("repository object with url", func(t *testing.T) {
		pkgData := map[string]interface{}{
			"repository": map[string]interface{}{
				"type": "git",
				"url":  "https://github.com/lodash/lodash.git",
			},
		}
		result := s.determineOrigin(pkgData, "lodash")
		assert.Equal(t, "https://github.com/lodash/lodash.git", result)
	})

	t.Run("repository string", func(t *testing.T) {
		pkgData := map[string]interface{}{
			"repository": "https://github.com/lodash/lodash",
		}
		result := s.determineOrigin(pkgData, "lodash")
		assert.Equal(t, "https://github.com/lodash/lodash", result)
	})

	t.Run("homepage fallback", func(t *testing.T) {
		pkgData := map[string]interface{}{
			"homepage": "https://lodash.com",
		}
		result := s.determineOrigin(pkgData, "lodash")
		assert.Equal(t, "https://lodash.com", result)
	})

	t.Run("npm prefix as last resort", func(t *testing.T) {
		pkgData := map[string]interface{}{}
		result := s.determineOrigin(pkgData, "lodash")
		assert.Equal(t, "npm:lodash", result)
	})
}

func TestNpmStrategy_YarnAliasPattern(t *testing.T) {
	tests := []struct {
		input    string
		alias    string
		realName string
	}{
		{
			`"my-alias@npm:@scope/real-pkg@^1.0.0":`,
			"my-alias",
			"@scope/real-pkg",
		},
		{
			`"alias@npm:real-pkg@^2.0.0":`,
			"alias",
			"real-pkg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := yarnAliasPattern.FindStringSubmatch(tt.input)
			require.NotNil(t, m)
			assert.Equal(t, tt.alias, m[1])
			assert.Equal(t, tt.realName, m[2])
		})
	}
}

func TestNpmStrategy_EnrichWithNpmRegistry(t *testing.T) {
	ctx, scm, fs, proc, http := newNpmTestDeps(t)

	// Set up registry response
	npmResp := map[string]interface{}{
		"name":    "lodash",
		"version": "4.17.21",
		"license": "MIT",
		"author":  "John-David Dalton",
		"repository": map[string]interface{}{
			"type": "git",
			"url":  "https://github.com/lodash/lodash.git",
		},
	}
	body, _ := json.Marshal(npmResp)
	http.Responses["https://registry.npmjs.org/lodash/4.17.21"] = &port.HTTPResponse{
		StatusCode: 200,
		Body:       body,
	}

	s := NewNpmStrategy(ctx, "https://github.com/org/test-npm-repo", scm, metadata.ScopeAll, nil, fs, proc, http)

	deps := map[string]string{"lodash": "4.17.21"}
	result := s.enrichWithNpmRegistry(nil, deps)

	require.Len(t, result, 1)
	assert.Equal(t, "lodash", result[0].Name)
	assert.Equal(t, "4.17.21", result[0].Version)
	assert.Equal(t, []string{"MIT"}, result[0].License)
	assert.Equal(t, []string{"John-David Dalton"}, result[0].Copyright)
	assert.Equal(t, "https://github.com/lodash/lodash.git", result[0].Origin)
}

func TestNpmStrategy_EnrichDoesNotOverwriteExisting(t *testing.T) {
	ctx, scm, fs, proc, http := newNpmTestDeps(t)

	npmResp := map[string]interface{}{
		"name":    "lodash",
		"version": "4.17.21",
		"license": "ISC",
		"author":  "Other Author",
	}
	body, _ := json.Marshal(npmResp)
	http.Responses["https://registry.npmjs.org/lodash/4.17.21"] = &port.HTTPResponse{
		StatusCode: 200,
		Body:       body,
	}

	s := NewNpmStrategy(ctx, "https://github.com/org/test-npm-repo", scm, metadata.ScopeAll, nil, fs, proc, http)

	existing := []metadata.Metadata{
		metadata.NewMetadata("lodash", "4.17.21", "https://github.com/lodash/lodash",
			"", []string{"MIT"}, []string{"John Doe"}),
	}
	deps := map[string]string{"lodash": "4.17.21"}
	result := s.enrichWithNpmRegistry(existing, deps)

	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/lodash/lodash", result[0].Origin)
	assert.Equal(t, []string{"MIT"}, result[0].License)
	assert.Equal(t, []string{"John Doe"}, result[0].Copyright)
}
