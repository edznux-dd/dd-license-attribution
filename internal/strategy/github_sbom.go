// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/strutil"
	"github.com/DataDog/dd-license-attribution/internal/urlutil"
)

// GitHubSBOMStrategy collects metadata from GitHub's dependency graph SBOM.
type GitHubSBOMStrategy struct {
	gh                    port.GitHubClient
	scm                   *artifact.SourceCodeManager
	withRootProject       bool
	withTransitiveDeps    bool
	splitter              *strutil.CustomSplit
	ctx                   context.Context
}

// NewGitHubSBOMStrategy creates a new GitHub SBOM strategy.
func NewGitHubSBOMStrategy(
	ctx context.Context,
	gh port.GitHubClient,
	scm *artifact.SourceCodeManager,
	scope metadata.ProjectScope,
	companySuffixes []string,
) *GitHubSBOMStrategy {
	withRoot := true
	withTransitive := true
	if scope == metadata.ScopeOnlyRootProject {
		withTransitive = false
	} else if scope == metadata.ScopeOnlyTransitiveDependencies {
		withRoot = false
	}
	return &GitHubSBOMStrategy{
		gh:                 gh,
		scm:                scm,
		withRootProject:    withRoot,
		withTransitiveDeps: withTransitive,
		splitter:           strutil.NewCustomSplit(companySuffixes),
		ctx:                ctx,
	}
}

func (s *GitHubSBOMStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	var updated []metadata.Metadata
	rootPackageIndex := 0

	for pkgIndex, pkg := range metas {
		if pkg.Origin == "" {
			updated = append(updated, pkg)
			continue
		}

		canonicalURL, apiURL := s.scm.GetCanonicalURLs(s.ctx, pkg.Origin)
		if apiURL == "" {
			updated = append(updated, pkg)
			continue
		}

		parsed := urlutil.ParseGitURL(canonicalURL)
		if !parsed.Valid || !parsed.GitHub {
			updated = append(updated, pkg)
			continue
		}

		owner := parsed.Owner
		repo := parsed.Repo
		pkg.Origin = canonicalURL

		// Update name to canonical format if it looks like a GitHub URL
		if pkg.Name != "" && (strings.HasPrefix(pkg.Name, "github.com/") ||
			strings.HasPrefix(pkg.Name, "https://github.com/") ||
			strings.HasPrefix(pkg.Name, "http://github.com/")) {
			pkg.Name = fmt.Sprintf("github.com/%s/%s", owner, repo)
		}

		canonicalPkgName := fmt.Sprintf("com.github.%s/%s", owner, repo)

		status, sbom, err := s.gh.GetSBOM(s.ctx, owner, repo)
		if err != nil || status != 200 || sbom == nil {
			if status == 404 {
				return nil, &artifact.NonAccessibleRepository{
					Msg: fmt.Sprintf("inexistent repository or private repository and not enough permissions for %s/%s", owner, repo),
				}
			}
			if status == 401 {
				return nil, &artifact.UnauthorizedRepository{
					Msg: fmt.Sprintf("GitHub token doesn't have enough permissions to access %s/%s", owner, repo),
				}
			}
			slog.Error("Failed to get SBOM", "owner", owner, "repo", repo, "status", status, "error", err)
			updated = append(updated, pkg)
			continue
		}

		packages := sbom.Packages

		// Filter by scope
		if !s.withRootProject {
			filtered := make([]port.GitHubSBOMPackage, 0, len(packages))
			for _, p := range packages {
				if p.Name != canonicalPkgName {
					filtered = append(filtered, p)
				}
			}
			packages = filtered
		}

		if !s.withTransitiveDeps {
			filtered := make([]port.GitHubSBOMPackage, 0)
			for _, p := range packages {
				for _, m := range metas {
					if m.Name != "" && strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(m.Name)) {
						filtered = append(filtered, p)
						break
					}
				}
			}
			packages = filtered
		}

		initialPackageProcessed := false
		isRootPackage := pkgIndex == rootPackageIndex

		for _, sbomPkg := range packages {
			// Skip GitHub Actions
			if strings.HasPrefix(sbomPkg.SPDXID, "SPDXRef-githubactions-") {
				continue
			}

			version, origin, license, copyright := "", "", []string{}, []string{}

			// Check existing metadata
			oldPkg := findByName(metas, sbomPkg.Name)
			newPkg := findByName(updated, sbomPkg.Name)

			if oldPkg != nil && newPkg == nil {
				version = oldPkg.Version
				origin = oldPkg.Origin
				license = oldPkg.License
				copyright = oldPkg.Copyright
			}
			if newPkg != nil {
				version = newPkg.Version
				origin = newPkg.Origin
				license = newPkg.License
				copyright = newPkg.Copyright
			}

			// Fill from SBOM
			if version == "" && sbomPkg.VersionInfo != "" && sbomPkg.VersionInfo != "NOASSERTION" {
				version = sbomPkg.VersionInfo
			}
			if origin == "" {
				if sbomPkg.DownloadLocation != "" && sbomPkg.DownloadLocation != "NOASSERTION" {
					origin = sbomPkg.DownloadLocation
				} else if strings.HasPrefix(sbomPkg.Name, "github.com") {
					origin = "https://" + sbomPkg.Name
				} else {
					origin = sbomPkg.Name
				}
			}
			if len(license) == 0 {
				if sbomPkg.LicenseDeclared != "" && sbomPkg.LicenseDeclared != "NOASSERTION" {
					license = []string{sbomPkg.LicenseDeclared}
				} else if sbomPkg.LicenseConcluded != "" && sbomPkg.LicenseConcluded != "NOASSERTION" {
					license = []string{sbomPkg.LicenseConcluded}
				}
			}
			if len(copyright) == 0 {
				if sbomPkg.CopyrightText != "" && sbomPkg.CopyrightText != "NOASSERTION" {
					copyright = uniqueStrings(s.splitter.Split(sbomPkg.CopyrightText, ','))
				}
			}

			if newPkg != nil {
				newPkg.Version = version
				newPkg.Origin = origin
				newPkg.License = license
				newPkg.Copyright = copyright
				if newPkg.Name == pkg.Name {
					initialPackageProcessed = true
				}
			} else {
				sbomIsRoot := sbomPkg.Name == canonicalPkgName
				if isRootPackage && sbomIsRoot {
					if version != "" {
						pkg.Version = version
					}
					if len(license) > 0 {
						pkg.License = license
					}
					if len(copyright) > 0 {
						pkg.Copyright = copyright
					}
					updated = append(updated, pkg)
					initialPackageProcessed = true
				} else {
					newMeta := metadata.NewMetadata(sbomPkg.Name, version, origin, "", license, copyright)
					updated = append(updated, newMeta)
					if newMeta.Name == pkg.Name {
						initialPackageProcessed = true
					}
				}
			}
		}

		if !initialPackageProcessed {
			if !s.withRootProject && isRootPackage {
				continue
			}
			updated = append(updated, pkg)
		}
	}

	return updated, nil
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}
