// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSCM(t *testing.T, gh port.GitHubClient) *artifact.SourceCodeManager {
	t.Helper()
	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{"/cache": true},
	}
	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, err := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)
	require.NoError(t, err)
	return scm
}

func TestGitHubSBOMStrategy_BasicSBOM(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-repo": {
				HTMLURL:       "https://github.com/DataDog/test-repo",
				URL:           "https://api.github.com/repos/DataDog/test-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-repo": 200},
		SBOMResponses: map[string]*port.GitHubSBOM{
			"DataDog/test-repo": {
				Packages: []port.GitHubSBOMPackage{
					{
						SPDXID:           "SPDXRef-pkg-a",
						Name:             "pkg-a",
						VersionInfo:      "1.0.0",
						DownloadLocation: "https://github.com/org/pkg-a",
						LicenseDeclared:  "MIT",
						CopyrightText:    "Org Inc",
					},
					{
						SPDXID:           "SPDXRef-root",
						Name:             "com.github.DataDog/test-repo",
						VersionInfo:      "0.1.0",
						DownloadLocation: "NOASSERTION",
						LicenseDeclared:  "Apache-2.0",
						CopyrightText:    "Datadog, Inc.",
					},
				},
			},
		},
		SBOMStatuses: map[string]int{"DataDog/test-repo": 200},
	}

	scm := newTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubSBOMStrategy(ctx, gh, scm, metadata.ScopeAll, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("github.com/DataDog/test-repo", "", "https://github.com/DataDog/test-repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)

	// Should have root + transitive dep
	assert.GreaterOrEqual(t, len(result), 2)

	// Find pkg-a
	var pkgA *metadata.Metadata
	for i := range result {
		if result[i].Name == "pkg-a" {
			pkgA = &result[i]
			break
		}
	}
	require.NotNil(t, pkgA)
	assert.Equal(t, "1.0.0", pkgA.Version)
	assert.Equal(t, []string{"MIT"}, pkgA.License)
}

func TestGitHubSBOMStrategy_404ReturnsError(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/private-repo": {
				HTMLURL:       "https://github.com/DataDog/private-repo",
				URL:           "https://api.github.com/repos/DataDog/private-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/private-repo": 200},
		SBOMStatuses: map[string]int{"DataDog/private-repo": 404},
	}

	scm := newTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubSBOMStrategy(ctx, gh, scm, metadata.ScopeAll, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("test", "", "https://github.com/DataDog/private-repo", "", nil, nil),
	}

	_, err := s.AugmentMetadata(metas)
	assert.Error(t, err)
	var nonAccessible *artifact.NonAccessibleRepository
	assert.ErrorAs(t, err, &nonAccessible)
}

func TestGitHubSBOMStrategy_401ReturnsError(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/secure-repo": {
				HTMLURL:       "https://github.com/DataDog/secure-repo",
				URL:           "https://api.github.com/repos/DataDog/secure-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/secure-repo": 200},
		SBOMStatuses: map[string]int{"DataDog/secure-repo": 401},
	}

	scm := newTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubSBOMStrategy(ctx, gh, scm, metadata.ScopeAll, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("test", "", "https://github.com/DataDog/secure-repo", "", nil, nil),
	}

	_, err := s.AugmentMetadata(metas)
	assert.Error(t, err)
	var unauthorized *artifact.UnauthorizedRepository
	assert.ErrorAs(t, err, &unauthorized)
}

func TestGitHubSBOMStrategy_SkipsEmptyOrigin(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
		SBOMResponses: map[string]*port.GitHubSBOM{},
	}

	scm := newTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubSBOMStrategy(ctx, gh, scm, metadata.ScopeAll, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("no-origin", "", "", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "no-origin", result[0].Name)
}

func TestGitHubSBOMStrategy_SkipsGitHubActions(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-repo": {
				HTMLURL:       "https://github.com/DataDog/test-repo",
				URL:           "https://api.github.com/repos/DataDog/test-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-repo": 200},
		SBOMResponses: map[string]*port.GitHubSBOM{
			"DataDog/test-repo": {
				Packages: []port.GitHubSBOMPackage{
					{
						SPDXID:          "SPDXRef-githubactions-checkout",
						Name:            "actions/checkout",
						LicenseDeclared: "MIT",
					},
					{
						SPDXID:           "SPDXRef-pkg",
						Name:             "real-pkg",
						VersionInfo:      "1.0",
						DownloadLocation: "https://example.com",
						LicenseDeclared:  "Apache-2.0",
					},
				},
			},
		},
		SBOMStatuses: map[string]int{"DataDog/test-repo": 200},
	}

	scm := newTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubSBOMStrategy(ctx, gh, scm, metadata.ScopeAll, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("test", "", "https://github.com/DataDog/test-repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)

	for _, m := range result {
		assert.NotEqual(t, "actions/checkout", m.Name, "GitHub Actions should be skipped")
	}
}

func TestGitHubSBOMStrategy_ScopeOnlyRootProject(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-repo": {
				HTMLURL:       "https://github.com/DataDog/test-repo",
				URL:           "https://api.github.com/repos/DataDog/test-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-repo": 200},
		SBOMResponses: map[string]*port.GitHubSBOM{
			"DataDog/test-repo": {
				Packages: []port.GitHubSBOMPackage{
					{
						SPDXID:           "SPDXRef-root",
						Name:             "com.github.DataDog/test-repo",
						VersionInfo:      "0.1.0",
						LicenseDeclared:  "Apache-2.0",
						DownloadLocation: "NOASSERTION",
					},
					{
						SPDXID:           "SPDXRef-dep",
						Name:             "some-dep",
						VersionInfo:      "2.0",
						DownloadLocation: "https://example.com",
						LicenseDeclared:  "MIT",
					},
				},
			},
		},
		SBOMStatuses: map[string]int{"DataDog/test-repo": 200},
	}

	scm := newTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubSBOMStrategy(ctx, gh, scm, metadata.ScopeOnlyRootProject, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("github.com/DataDog/test-repo", "", "https://github.com/DataDog/test-repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)

	// Only root package (and any pre-existing that match the name filter) should be present
	for _, m := range result {
		// "some-dep" should not appear since it's not in the input metas (scope filtering)
		if m.Name == "some-dep" {
			// Check it was in the filtered set - since onlyRootProject means withTransitiveDeps=false,
			// it only includes packages whose name matches something in metas
			found := false
			for _, orig := range metas {
				if orig.Name != "" && m.Name == orig.Name {
					found = true
				}
			}
			if !found {
				t.Errorf("transitive dep 'some-dep' should be filtered out with ScopeOnlyRootProject")
			}
		}
	}
}
