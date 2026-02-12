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

func TestLicenseScanStrategy_DetectsFromLocalPath(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{
			"/cache":         true,
			"/src/my-pkg":    true,
		},
		Files: map[string][]byte{
			"/src/my-pkg/LICENSE": []byte("MIT License..."),
		},
	}

	detector := &testutil.MockLicenseDetector{
		Licenses: map[string][]port.DetectedLicense{
			"/src/my-pkg": {{Name: "MIT", Confidence: 0.95}},
		},
		Copyrights: map[string][]port.DetectedCopyright{
			"/src/my-pkg/LICENSE": {{Holder: "Test Author"}},
		},
	}

	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, err := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)
	require.NoError(t, err)

	ctx := context.Background()
	s := NewLicenseScanStrategy(ctx, scm, detector, fs, nil, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("my-pkg", "1.0", "https://github.com/org/my-pkg", "/src/my-pkg", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, []string{"MIT"}, result[0].License)
	assert.Equal(t, []string{"Test Author"}, result[0].Copyright)
}

func TestLicenseScanStrategy_SkipsIfAlreadyHasData(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{"/cache": true},
	}
	detector := &testutil.MockLicenseDetector{
		Licenses: map[string][]port.DetectedLicense{},
	}
	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, err := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)
	require.NoError(t, err)

	ctx := context.Background()
	s := NewLicenseScanStrategy(ctx, scm, detector, fs, nil, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("my-pkg", "1.0", "https://github.com/org/my-pkg", "/src/my-pkg",
			[]string{"Apache-2.0"}, []string{"Existing Author"}),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, []string{"Apache-2.0"}, result[0].License)
	assert.Equal(t, []string{"Existing Author"}, result[0].Copyright)
}

func TestLicenseScanStrategy_SkipsEmptyOrigin(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{"/cache": true},
	}
	detector := &testutil.MockLicenseDetector{}
	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, _ := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)

	ctx := context.Background()
	s := NewLicenseScanStrategy(ctx, scm, detector, fs, nil, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("no-origin", "", "", "", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "no-origin", result[0].Name)
}

func TestLicenseScanStrategy_CopyrightFileFilter(t *testing.T) {
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{
			"/cache":      true,
			"/src/my-pkg": true,
		},
		Files: map[string][]byte{
			"/src/my-pkg/LICENSE":    []byte("MIT License..."),
			"/src/my-pkg/README.md":  []byte("Some readme"),
			"/src/my-pkg/NOTICE":     []byte("Some notice"),
		},
	}

	detector := &testutil.MockLicenseDetector{
		Licenses: map[string][]port.DetectedLicense{
			"/src/my-pkg": {{Name: "MIT", Confidence: 0.95}},
		},
		Copyrights: map[string][]port.DetectedCopyright{
			"/src/my-pkg/LICENSE": {{Holder: "License Author"}},
			"/src/my-pkg/NOTICE":  {{Holder: "Notice Author"}},
			"/src/my-pkg/README.md": {{Holder: "Readme Author"}},
		},
	}

	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, _ := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)

	ctx := context.Background()
	// Only scan LICENSE files for copyright
	s := NewLicenseScanStrategy(ctx, scm, detector, fs, nil, []string{"LICENSE"})

	metas := []metadata.Metadata{
		metadata.NewMetadata("my-pkg", "1.0", "https://github.com/org/my-pkg", "/src/my-pkg", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)

	// Only LICENSE file should be scanned for copyright
	assert.Equal(t, []string{"License Author"}, result[0].Copyright)
}

func TestLicenseScanStrategy_CopyrightPriority(t *testing.T) {
	// Tests the priority: holders > authors > statements
	gh := &testutil.MockGitHubClient{
		RepoResponses: map[string]*port.GitHubRepository{},
	}

	fs := &testutil.MockFileSystem{
		Dirs: map[string]bool{
			"/cache":      true,
			"/src/my-pkg": true,
		},
		Files: map[string][]byte{
			"/src/my-pkg/LICENSE": []byte("MIT License..."),
		},
	}

	detector := &testutil.MockLicenseDetector{
		Licenses: map[string][]port.DetectedLicense{
			"/src/my-pkg": {{Name: "MIT"}},
		},
		Copyrights: map[string][]port.DetectedCopyright{
			"/src/my-pkg/LICENSE": {
				{Author: "Author Name"},
				{Statement: "Copyright 2024 Statement Corp"},
			},
		},
	}

	proc := &testutil.MockProcessRunner{Outputs: map[string]string{}}
	clock := &testutil.MockClock{FixedTime: time.Now()}
	scm, _ := artifact.NewSourceCodeManager("/cache", gh, fs, proc, clock, 3600, nil)

	ctx := context.Background()
	s := NewLicenseScanStrategy(ctx, scm, detector, fs, nil, nil)

	metas := []metadata.Metadata{
		metadata.NewMetadata("my-pkg", "1.0", "https://github.com/org/my-pkg", "/src/my-pkg", nil, nil),
	}

	result, err := s.AugmentMetadata(metas)
	require.NoError(t, err)
	require.Len(t, result, 1)
	// No holders, so should fall back to authors
	assert.Equal(t, []string{"Author Name"}, result[0].Copyright)
}
