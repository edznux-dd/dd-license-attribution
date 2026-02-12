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

func TestGoPkgStrategy_BasicGoList(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{
			"DataDog/test-go-repo": {
				HTMLURL:       "https://github.com/DataDog/test-go-repo",
				URL:           "https://api.github.com/repos/DataDog/test-go-repo",
				DefaultBranch: "main",
				Owner:         port.GitHubOwner{Login: "DataDog"},
			},
		},
		RepoStatuses: map[string]int{"DataDog/test-go-repo": 200},
	}

	goListOutput := `{
  "Module": {
    "Path": "github.com/DataDog/test-go-repo",
    "Version": "v0.1.0",
    "Dir": "/tmp/go-repo"
  }
}
{
  "Module": {
    "Path": "github.com/stretchr/testify",
    "Version": "v1.9.0",
    "Dir": "/tmp/testify"
  }
}`

	clonePath := "/cache/" + time.Now().Format(artifact.CacheTimestampFormat) + "/DataDog-test-go-repo/default_branch"
	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{
			"/cache":    true,
			clonePath:   true,
		},
		Files: map[string][]byte{
			clonePath + "/go.mod": []byte(`module github.com/DataDog/test-go-repo

go 1.22

require github.com/stretchr/testify v1.9.0
`),
		},
	}

	proc := &testutil.MockProcessRunner{
		Outputs: map[string]string{
			// For discoverDefaultBranch in getMirrorURLAndRef
			"git ls-remote --symref https://github.com/DataDog/test-go-repo HEAD": "ref: refs/heads/main\tHEAD\n",
			// For git clone
			"git clone -c advice.detachedHead=False --depth 1 --branch=main https://github.com/DataDog/test-go-repo " + clonePath: "",
			// For go list
			clonePath + ":go list -json all": goListOutput,
		},
	}

	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, err := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)
	require.NoError(t, err)

	ctx := context.Background()
	s := NewGoPkgStrategy(ctx, "https://github.com/DataDog/test-go-repo", scm, metadata.ScopeAll, fs, proc)

	metas := []metadata.Metadata{
		metadata.NewMetadata("github.com/DataDog/test-go-repo", "", "https://github.com/DataDog/test-go-repo", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result), 1)

	// Check the root package was updated
	var rootPkg *metadata.Metadata
	for i := range result {
		if result[i].Name == "github.com/DataDog/test-go-repo" {
			rootPkg = &result[i]
			break
		}
	}
	require.NotNil(t, rootPkg)
}

func TestGoPkgStrategy_TranslateGitHubPath(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{"/cache": true},
	}
	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, _ := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)

	ctx := context.Background()
	s := NewGoPkgStrategy(ctx, "https://github.com/test/repo", scm, metadata.ScopeAll, fs, proc)

	// Non-GitHub paths
	assert.Equal(t, "https://golang.org/x/sync", s.translateGitHubPath("golang.org/x/sync"))

	// GitHub with just 3 parts
	assert.Equal(t, "https://github.com/stretchr/testify", s.translateGitHubPath("github.com/stretchr/testify"))
}

func TestGoPkgStrategy_IsExamplePackage(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	mainGoMod := `module github.com/DataDog/test-repo/examples

go 1.22

require github.com/DataDog/test-repo v0.1.0
`
	normalGoMod := `module github.com/DataDog/test-repo

go 1.22
`

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{"/cache": true},
		Files: map[string][]byte{
			"/example/go.mod": []byte(mainGoMod),
			"/normal/go.mod":  []byte(normalGoMod),
		},
	}
	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, _ := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)

	ctx := context.Background()
	s := NewGoPkgStrategy(ctx, "https://github.com/DataDog/test-repo", scm, metadata.ScopeAll, fs, proc)

	assert.True(t, s.isExamplePackage("/example"))
	assert.False(t, s.isExamplePackage("/normal"))
}
