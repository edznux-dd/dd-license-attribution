// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
)

// GoPkgStrategy collects Go package metadata via `go list -json all`.
type GoPkgStrategy struct {
	topPackage      string
	scm             *artifact.SourceCodeManager
	onlyRootProject bool
	headBranchCache map[string]string
	fs              port.FileSystem
	proc            port.ProcessRunner
	ctx             context.Context
}

// NewGoPkgStrategy creates a GoPkg strategy.
func NewGoPkgStrategy(
	ctx context.Context,
	topPackage string,
	scm *artifact.SourceCodeManager,
	scope metadata.ProjectScope,
	fsys port.FileSystem,
	proc port.ProcessRunner,
) *GoPkgStrategy {
	canonical, _ := scm.GetCanonicalURLs(ctx, topPackage)
	if canonical != "" {
		topPackage = canonical
	}
	return &GoPkgStrategy{
		topPackage:      topPackage,
		scm:             scm,
		onlyRootProject: scope == metadata.ScopeOnlyRootProject,
		headBranchCache: make(map[string]string),
		fs:              fsys,
		proc:            proc,
		ctx:             ctx,
	}
}

var moduleLineRegex = regexp.MustCompile(`^module\s+(\S+)`)
var requireLineRegex = regexp.MustCompile(`^require\s+(\S+)\s+v[\d.]+`)

func (s *GoPkgStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	ref, err := s.scm.GetCode(s.ctx, s.topPackage, false)
	if err != nil || ref == nil {
		return metas, nil
	}

	// Walk to find go.mod files
	err = s.fs.WalkDir(ref.LocalFullPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Name() != "go.mod" {
			return nil
		}

		dir := strings.TrimSuffix(path, "/go.mod")
		if s.isExamplePackage(dir) {
			return nil
		}

		output, err := s.proc.OutputInDir(s.ctx, dir, "go", "list", "-json", "all")
		if err != nil {
			slog.Warn("Failed to run go list", "dir", dir, "error", err)
			return nil
		}

		// Fix output: go list -json produces multiple JSON objects
		corrected := "[" + strings.ReplaceAll(output, "}\n{", "},\n{") + "]"
		var pkgList []goListPackage
		if err := json.Unmarshal([]byte(corrected), &pkgList); err != nil {
			slog.Warn("Failed to parse go list output", "dir", dir, "error", err)
			return nil
		}

		for _, pkg := range pkgList {
			if pkg.Module == nil {
				continue
			}
			if s.onlyRootProject {
				found := false
				for _, m := range metas {
					if m.Name == pkg.Module.Path {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			version := pkg.Module.Version
			origin := s.translateGitHubPath(pkg.Module.Path)

			pkgMeta := metadata.NewMetadata(
				pkg.Module.Path, version, origin,
				pkg.Module.Dir, nil, nil,
			)

			updated := false
			for i, m := range metas {
				if m.Name == pkgMeta.Name {
					metas[i].Origin = pkgMeta.Origin
					metas[i].LocalSrcPath = pkgMeta.LocalSrcPath
					if pkgMeta.Version != "" {
						metas[i].Version = pkgMeta.Version
					}
					updated = true
					break
				}
			}
			if !updated {
				metas = append(metas, pkgMeta)
			}
		}
		return nil
	})

	return metas, err
}

type goListPackage struct {
	Module *goListModule `json:"Module"`
}

type goListModule struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
}

func (s *GoPkgStrategy) translateGitHubPath(path string) string {
	if !strings.HasPrefix(path, "github.com") {
		return "https://" + path
	}
	parts := strings.SplitN(path, "/", 4)
	if len(parts) > 3 {
		repoURL := fmt.Sprintf("https://%s/%s/%s", parts[0], parts[1], parts[2])
		branch := s.getHeadBranch(repoURL)
		return fmt.Sprintf("%s/tree/%s/%s", repoURL, branch, parts[3])
	}
	return "https://" + path
}

func (s *GoPkgStrategy) getHeadBranch(repoURL string) string {
	if cached, ok := s.headBranchCache[repoURL]; ok {
		return cached
	}
	out, err := s.proc.Output(s.ctx, "git", "ls-remote", "--symref", repoURL, "HEAD")
	if err != nil {
		s.headBranchCache[repoURL] = "main"
		return "main"
	}
	fields := strings.Fields(out)
	if len(fields) >= 2 {
		branch := strings.TrimPrefix(fields[1], "refs/heads/")
		s.headBranchCache[repoURL] = branch
		return branch
	}
	s.headBranchCache[repoURL] = "main"
	return "main"
}

func (s *GoPkgStrategy) isExamplePackage(goModPath string) bool {
	data, err := s.fs.ReadFile(goModPath + "/go.mod")
	if err != nil {
		return false
	}

	var moduleName string
	requiresMainModule := false

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if m := moduleLineRegex.FindStringSubmatch(line); m != nil {
			moduleName = m[1]
		}
		if m := requireLineRegex.FindStringSubmatch(line); m != nil {
			if moduleName != "" && strings.Contains(moduleName, m[1]) {
				requiresMainModule = true
			}
		}
	}

	return moduleName != "" && strings.HasSuffix(moduleName, "/examples") && requiresMainModule
}
