// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package testutil

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/DataDog/dd-license-attribution/internal/port"
)

// MockFileSystem implements port.FileSystem for tests.
type MockFileSystem struct {
	Files map[string][]byte
	Dirs  map[string]bool // directories that exist
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	if data, ok := m.Files[path]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("file not found: %s", path)
}

func (m *MockFileSystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if m.Files == nil {
		m.Files = make(map[string][]byte)
	}
	m.Files[path] = data
	return nil
}

func (m *MockFileSystem) Exists(path string) bool {
	if _, ok := m.Files[path]; ok {
		return true
	}
	if m.Dirs != nil {
		if _, ok := m.Dirs[path]; ok {
			return true
		}
	}
	return false
}

func (m *MockFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	if m.Dirs == nil {
		m.Dirs = make(map[string]bool)
	}
	m.Dirs[path] = true
	return nil
}

func (m *MockFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	// Return entries for files that are under this path
	var entries []fs.DirEntry
	prefix := path + "/"
	seen := make(map[string]bool)
	for p := range m.Files {
		if strings.HasPrefix(p, prefix) {
			rest := strings.TrimPrefix(p, prefix)
			parts := strings.SplitN(rest, "/", 2)
			name := parts[0]
			if !seen[name] {
				seen[name] = true
				isDir := len(parts) > 1
				entries = append(entries, &mockDirEntry{name: name, isDir: isDir})
			}
		}
	}
	return entries, nil
}

func (m *MockFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	// Simple walk implementation for mock
	for path, data := range m.Files {
		if strings.HasPrefix(path, root) {
			entry := &mockDirEntry{name: filepath.Base(path), isDir: false, size: int64(len(data))}
			if err := fn(path, entry, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

type mockDirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (e *mockDirEntry) Name() string               { return e.name }
func (e *mockDirEntry) IsDir() bool                 { return e.isDir }
func (e *mockDirEntry) Type() fs.FileMode           { return 0 }
func (e *mockDirEntry) Info() (fs.FileInfo, error)   { return nil, nil }

// MockProcessRunner implements port.ProcessRunner for tests.
type MockProcessRunner struct {
	Outputs map[string]string // command -> output
	Errors  map[string]error  // command -> error
}

func (m *MockProcessRunner) Run(ctx context.Context, name string, args ...string) (int, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := m.Errors[key]; ok {
		return 1, err
	}
	return 0, nil
}

func (m *MockProcessRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if err, ok := m.Errors[key]; ok {
		return "", err
	}
	if out, ok := m.Outputs[key]; ok {
		return out, nil
	}
	return "", nil
}

func (m *MockProcessRunner) OutputInDir(ctx context.Context, dir string, name string, args ...string) (string, error) {
	key := dir + ":" + name + " " + strings.Join(args, " ")
	if err, ok := m.Errors[key]; ok {
		return "", err
	}
	if out, ok := m.Outputs[key]; ok {
		return out, nil
	}
	// Fall back to non-dir key
	return m.Output(ctx, name, args...)
}

func (m *MockProcessRunner) RunInDir(ctx context.Context, dir string, name string, args ...string) (int, error) {
	key := dir + ":" + name + " " + strings.Join(args, " ")
	if err, ok := m.Errors[key]; ok {
		return 1, err
	}
	return 0, nil
}

// MockGitHubClient implements port.GitHubClient for tests.
type MockGitHubClient struct {
	SBOMResponses map[string]*port.GitHubSBOM // "owner/repo" -> SBOM
	SBOMStatuses  map[string]int
	RepoResponses map[string]*port.GitHubRepository
	RepoStatuses  map[string]int
}

func (m *MockGitHubClient) GetSBOM(ctx context.Context, owner, repo string) (int, *port.GitHubSBOM, error) {
	key := owner + "/" + repo
	status := 200
	if s, ok := m.SBOMStatuses[key]; ok {
		status = s
	}
	sbom := m.SBOMResponses[key]
	return status, sbom, nil
}

func (m *MockGitHubClient) GetRepository(ctx context.Context, owner, repo string) (int, *port.GitHubRepository, error) {
	key := owner + "/" + repo
	status := 200
	if s, ok := m.RepoStatuses[key]; ok {
		status = s
	}
	repository := m.RepoResponses[key]
	return status, repository, nil
}

// MockHTTPFetcher implements port.HTTPFetcher for tests.
type MockHTTPFetcher struct {
	Responses map[string]*port.HTTPResponse // URL -> response
	Errors    map[string]error
}

func (m *MockHTTPFetcher) Get(ctx context.Context, url string) (*port.HTTPResponse, error) {
	if err, ok := m.Errors[url]; ok {
		return nil, err
	}
	if resp, ok := m.Responses[url]; ok {
		return resp, nil
	}
	return &port.HTTPResponse{StatusCode: 404, Body: nil}, nil
}

// MockLicenseDetector implements port.LicenseDetector for tests.
type MockLicenseDetector struct {
	Licenses   map[string][]port.DetectedLicense   // dir -> licenses
	Copyrights map[string][]port.DetectedCopyright  // file -> copyrights
}

func (m *MockLicenseDetector) DetectLicenses(dir string) ([]port.DetectedLicense, error) {
	if lics, ok := m.Licenses[dir]; ok {
		return lics, nil
	}
	return nil, nil
}

func (m *MockLicenseDetector) DetectCopyrights(files []string) ([]port.DetectedCopyright, error) {
	var result []port.DetectedCopyright
	for _, f := range files {
		if cs, ok := m.Copyrights[f]; ok {
			result = append(result, cs...)
		}
	}
	return result, nil
}

// MockClock implements port.Clock for tests.
type MockClock struct {
	FixedTime time.Time
}

func (m *MockClock) Now() time.Time {
	return m.FixedTime
}

// MockLLMClient implements port.LLMClient for tests.
type MockLLMClient struct {
	Responses map[string]string // license text -> SPDX expression
	Errors    map[string]error
}

func (m *MockLLMClient) ConvertToSPDX(ctx context.Context, licenseText string) (string, error) {
	if err, ok := m.Errors[licenseText]; ok {
		return "", err
	}
	if resp, ok := m.Responses[licenseText]; ok {
		return resp, nil
	}
	return "UNKNOWN", nil
}

// MockUserPrompter implements port.UserPrompter for tests.
type MockUserPrompter struct {
	ConfirmResponses []bool
	PromptResponses  []string
	confirmIdx       int
	promptIdx        int
}

func (m *MockUserPrompter) Confirm(msg string) (bool, error) {
	if m.confirmIdx >= len(m.ConfirmResponses) {
		return false, fmt.Errorf("no more confirm responses")
	}
	resp := m.ConfirmResponses[m.confirmIdx]
	m.confirmIdx++
	return resp, nil
}

func (m *MockUserPrompter) Prompt(msg string, defaultVal string) (string, error) {
	if m.promptIdx >= len(m.PromptResponses) {
		return defaultVal, nil
	}
	resp := m.PromptResponses[m.promptIdx]
	m.promptIdx++
	return resp, nil
}
