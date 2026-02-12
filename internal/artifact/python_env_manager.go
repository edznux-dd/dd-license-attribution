// SPDX-License-Identifier: Apache-2.0
package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/DataDog/dd-license-attribution/internal/port"
)

// PyEnvRuntimeError indicates a Python environment setup failure.
type PyEnvRuntimeError struct{ Msg string }

func (e *PyEnvRuntimeError) Error() string { return e.Msg }

// PythonEnvManager manages Python virtual environments for dependency analysis.
type PythonEnvManager struct {
	cacheDir       string
	cacheTTL       int
	setupTime      time.Time
	timestampedDir string
	fs             port.FileSystem
	proc           port.ProcessRunner
	clock          port.Clock
}

// NewPythonEnvManager creates a PythonEnvManager.
func NewPythonEnvManager(
	cacheDir string,
	fs port.FileSystem,
	proc port.ProcessRunner,
	clock port.Clock,
	cacheTTL int,
) *PythonEnvManager {
	now := clock.Now()
	return &PythonEnvManager{
		cacheDir:       cacheDir,
		cacheTTL:       cacheTTL,
		setupTime:      now,
		timestampedDir: now.Format(CacheTimestampFormat),
		fs:             fs,
		proc:           proc,
		clock:          clock,
	}
}

var pythonProjectFiles = []string{
	"requirements.txt",
	"setup.py",
	"setup.cfg",
	"pyproject.toml",
	"Pipfile",
	"Pipfile.lock",
}

// GetEnvironment creates or retrieves a cached Python venv for the given path.
func (m *PythonEnvManager) GetEnvironment(ctx context.Context, resourcePath string) (string, error) {
	entries, err := m.fs.ReadDir(resourcePath)
	if err != nil {
		return "", nil
	}

	isPythonProject := false
	fileNames := make(map[string]bool)
	for _, e := range entries {
		fileNames[e.Name()] = true
	}
	for _, pf := range pythonProjectFiles {
		if fileNames[pf] {
			isPythonProject = true
			break
		}
	}
	if !isPythonProject {
		return "", nil
	}

	normalizedPath := strings.ReplaceAll(resourcePath, "/", "_")
	cached := m.getCached(normalizedPath)
	if cached != "" {
		return cached, nil
	}

	venvPath, err := m.createPythonEnv(ctx, resourcePath, normalizedPath)
	if err != nil {
		return "", err
	}

	if err := m.installDependencies(ctx, resourcePath, venvPath); err != nil {
		return "", err
	}

	return venvPath, nil
}

func (m *PythonEnvManager) createPythonEnv(ctx context.Context, resourcePath, normalizedPath string) (string, error) {
	venvPath := fmt.Sprintf("%s/%s/%s_virtualenv", m.cacheDir, m.timestampedDir, normalizedPath)
	code, err := m.proc.RunInDir(ctx, resourcePath, "python", "-m", "venv", venvPath)
	if err != nil || code != 0 {
		return "", &PyEnvRuntimeError{Msg: "failed to create Python virtualenv"}
	}
	return venvPath, nil
}

func (m *PythonEnvManager) installDependencies(ctx context.Context, resourcePath, venvPath string) error {
	code, err := m.proc.RunInDir(ctx, resourcePath, venvPath+"/bin/python", "-m", "pip", "install", ".")
	if err != nil || code != 0 {
		return &PyEnvRuntimeError{Msg: "failed to install dependencies when creating Python virtualenv cache"}
	}
	return nil
}

func (m *PythonEnvManager) getCached(normalizedPath string) string {
	entries, err := m.fs.ReadDir(m.cacheDir)
	if err != nil {
		return ""
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	threshold := m.setupTime.Add(-time.Duration(m.cacheTTL) * time.Second)
	for _, name := range names {
		ts, err := time.Parse(CacheTimestampFormat, name)
		if err != nil {
			continue
		}
		if ts.Before(threshold) {
			continue
		}
		venvPath := fmt.Sprintf("%s/%s/%s_virtualenv", m.cacheDir, name, normalizedPath)
		if m.fs.Exists(venvPath) {
			return venvPath
		}
	}
	return ""
}

// GetDependencies reads installed packages from a pip environment.
func GetDependencies(ctx context.Context, proc port.ProcessRunner, venvPath string) ([][2]string, error) {
	out, err := proc.Output(ctx, venvPath+"/bin/pip", "list", "--format=json")
	if err != nil {
		return nil, fmt.Errorf("failed to list pip packages: %w", err)
	}

	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(out), &packages); err != nil {
		return nil, fmt.Errorf("failed to parse pip output: %w", err)
	}

	deps := make([][2]string, len(packages))
	for i, p := range packages {
		deps[i] = [2]string{p.Name, p.Version}
		slog.Debug("Dependency", "name", p.Name, "version", p.Version)
	}
	return deps, nil
}
