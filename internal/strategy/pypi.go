// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/strutil"
	"github.com/DataDog/dd-license-attribution/internal/urlutil"
)

// PyPIStrategy collects metadata from PyPI for Python dependencies.
type PyPIStrategy struct {
	topPackage      string
	scm             *artifact.SourceCodeManager
	pyEnv           *artifact.PythonEnvManager
	onlyRootProject bool
	splitter        *strutil.CustomSplit
	http            port.HTTPFetcher
	proc            port.ProcessRunner
	ctx             context.Context
}

// NewPyPIStrategy creates a new PyPI strategy.
func NewPyPIStrategy(
	ctx context.Context,
	topPackage string,
	scm *artifact.SourceCodeManager,
	pyEnv *artifact.PythonEnvManager,
	scope metadata.ProjectScope,
	companySuffixes []string,
	http port.HTTPFetcher,
	proc port.ProcessRunner,
) *PyPIStrategy {
	canonical, _ := scm.GetCanonicalURLs(ctx, topPackage)
	if canonical != "" {
		topPackage = canonical
	}
	return &PyPIStrategy{
		topPackage:      topPackage,
		scm:             scm,
		pyEnv:           pyEnv,
		onlyRootProject: scope == metadata.ScopeOnlyRootProject,
		splitter:        strutil.NewCustomSplit(companySuffixes),
		http:            http,
		proc:            proc,
		ctx:             ctx,
	}
}

func (s *PyPIStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	result := make([]metadata.Metadata, len(metas))
	copy(result, metas)

	topPath := s.findTopMetadataPath(result)
	if topPath == "" {
		return result, nil
	}

	venvPath, err := s.pyEnv.GetEnvironment(s.ctx, topPath)
	if err != nil {
		return nil, err
	}
	if venvPath == "" {
		return result, nil
	}

	deps, err := artifact.GetDependencies(s.ctx, s.proc, venvPath)
	if err != nil {
		return result, nil
	}

	for _, dep := range deps {
		depName, version := dep[0], dep[1]

		pypiMeta, err := s.getMetadataFromPyPI(depName, version)
		if err != nil || pypiMeta == nil {
			continue
		}

		info := pypiMeta.Info
		origin := "pypi:" + depName

		// Extract origin from project URLs
		if info.ProjectURLs != nil {
			urls := info.ProjectURLs
			urlsLower := make(map[string]string)
			for k, v := range urls {
				if v != "" {
					v = strings.Replace(v, "http://", "https://", 1)
					urlsLower[strings.ToLower(k)] = v
				}
			}

			// Priority order for origin
			for _, key := range []string{"source", "source code", "code", "repository", "github", "homepage"} {
				if u, ok := urlsLower[key]; ok && urlutil.IsValidGitURL(u) {
					origin = u
				}
			}
		}

		// Find existing or create new
		sbomName := translateNameGHtoPyPI(info.Name)
		existing := findByName(result, info.Name)
		if existing == nil {
			existing = findByName(result, sbomName)
		}

		if existing != nil {
			if existing.Origin == "" || !urlutil.IsValidGitURL(existing.Origin) {
				existing.Origin = origin
			}
			if len(existing.License) == 0 && info.License != "" {
				if strutil.IsLongLicense(info.License) {
					existing.License = []string{info.License}
				} else {
					existing.License = strings.Split(info.License, ",")
				}
			}
			if info.Version != "" {
				existing.Version = info.Version
			}
			if len(existing.Copyright) == 0 && info.Author != "" {
				existing.Copyright = uniqueStrings(s.splitter.Split(info.Author, ','))
			}
		} else {
			var license []string
			if info.License != "" {
				if strutil.IsLongLicense(info.License) {
					license = []string{info.License}
				} else {
					license = strings.Split(info.License, ",")
				}
			}
			var copyright []string
			if info.Author != "" {
				copyright = s.splitter.Split(info.Author, ',')
			}
			result = append(result, metadata.NewMetadata(
				info.Name, info.Version, origin, "",
				license, copyright,
			))
		}
	}
	return result, nil
}

type pypiResponse struct {
	Info pypiInfo `json:"info"`
}

type pypiInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	License     string            `json:"license"`
	Author      string            `json:"author"`
	ProjectURLs map[string]string `json:"project_urls"`
}

func (s *PyPIStrategy) getMetadataFromPyPI(pkg, version string) (*pypiResponse, error) {
	url := "https://pypi.org/pypi/" + pkg + "/" + version + "/json"
	resp, err := s.http.Get(s.ctx, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 || resp.StatusCode == 503 {
		slog.Warn("PyPI returned error", "package", pkg, "status", resp.StatusCode)
		return nil, nil
	}

	var result pypiResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func translateNameGHtoPyPI(name string) string {
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = strings.ReplaceAll(name, "github.com/", "com.github.")
	return name
}

func (s *PyPIStrategy) findTopMetadataPath(metas []metadata.Metadata) string {
	translatedName := translateNameGHtoPyPI(s.topPackage)
	for i := range metas {
		if metas[i].Name == s.topPackage || metas[i].Name == translatedName {
			if metas[i].LocalSrcPath != "" {
				return metas[i].LocalSrcPath
			}
			if metas[i].Origin != "" {
				ref, err := s.scm.GetCode(s.ctx, metas[i].Origin, false)
				if err == nil && ref != nil {
					metas[i].LocalSrcPath = ref.LocalFullPath
					return ref.LocalFullPath
				}
			}
		}
		if metas[i].Origin != "" && strings.HasSuffix(metas[i].Origin, s.topPackage) {
			ref, err := s.scm.GetCode(s.ctx, metas[i].Origin, false)
			if err == nil && ref != nil {
				metas[i].LocalSrcPath = ref.LocalFullPath
				return ref.LocalFullPath
			}
		}
	}
	return ""
}
