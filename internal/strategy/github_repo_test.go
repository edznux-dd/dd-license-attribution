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

func TestGitHubRepoStrategy_FillsMissingLicenseAndCopyright(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-repo": {
				HTMLURL:       "https://github.com/DataDog/test-repo",
				URL:           "https://api.github.com/repos/DataDog/test-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
				License:       &port.GitHubLicense{SPDXID: "Apache-2.0"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-repo": 200},
	}

	scm := newRepoTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubRepoStrategy(ctx, gh, scm)

	metas := []metadata.Metadata{
		metadata.NewMetadata("test-repo", "", "https://github.com/DataDog/test-repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/DataDog/test-repo", result[0].Origin)
	assert.Equal(t, []string{"Apache-2.0"}, result[0].License)
	assert.Equal(t, []string{"DataDog"}, result[0].Copyright)
}

func TestGitHubRepoStrategy_DoesNotOverwriteExisting(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-repo": {
				HTMLURL:       "https://github.com/DataDog/test-repo",
				URL:           "https://api.github.com/repos/DataDog/test-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
				License:       &port.GitHubLicense{SPDXID: "MIT"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-repo": 200},
	}

	scm := newRepoTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubRepoStrategy(ctx, gh, scm)

	metas := []metadata.Metadata{
		metadata.NewMetadata("test-repo", "", "https://github.com/DataDog/test-repo", "",
			[]string{"Apache-2.0"}, []string{"Existing Author"}),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, []string{"Apache-2.0"}, result[0].License)
	assert.Equal(t, []string{"Existing Author"}, result[0].Copyright)
}

func TestGitHubRepoStrategy_SkipsEmptyOrigin(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	scm := newRepoTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubRepoStrategy(ctx, gh, scm)

	metas := []metadata.Metadata{
		metadata.NewMetadata("no-origin", "", "", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "no-origin", result[0].Name)
}

func TestGitHubRepoStrategy_SkipsNonGitHub(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	scm := newRepoTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubRepoStrategy(ctx, gh, scm)

	metas := []metadata.Metadata{
		metadata.NewMetadata("gitlab-pkg", "", "https://gitlab.com/org/repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "gitlab-pkg", result[0].Name)
}

func TestGitHubRepoStrategy_NOASSERTIONLicense(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-repo": {
				HTMLURL:       "https://github.com/DataDog/test-repo",
				URL:           "https://api.github.com/repos/DataDog/test-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
				License:       &port.GitHubLicense{SPDXID: "NOASSERTION"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-repo": 200},
	}

	scm := newRepoTestSCM(t, gh)
	ctx := context.Background()
	s := NewGitHubRepoStrategy(ctx, gh, scm)

	metas := []metadata.Metadata{
		metadata.NewMetadata("test-repo", "", "https://github.com/DataDog/test-repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, []string{}, result[0].License)
	assert.Equal(t, []string{"DataDog"}, result[0].Copyright)
}

func newRepoTestSCM(t *testing.T, gh port.GitHubClient) *artifact.SourceCodeManager {
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
