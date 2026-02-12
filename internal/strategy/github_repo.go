// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/urlutil"
)

// GitHubRepoStrategy collects metadata from GitHub repository API.
type GitHubRepoStrategy struct {
	gh  port.GitHubClient
	scm *artifact.SourceCodeManager
	ctx context.Context
}

// NewGitHubRepoStrategy creates a new GitHub repository strategy.
func NewGitHubRepoStrategy(
	ctx context.Context,
	gh port.GitHubClient,
	scm *artifact.SourceCodeManager,
) *GitHubRepoStrategy {
	return &GitHubRepoStrategy{gh: gh, scm: scm, ctx: ctx}
}

func (s *GitHubRepoStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	var result []metadata.Metadata

	for _, pkg := range metas {
		if pkg.Origin == "" {
			result = append(result, pkg)
			continue
		}

		canonicalURL, apiURL := s.scm.GetCanonicalURLs(s.ctx, pkg.Origin)
		if apiURL == "" {
			result = append(result, pkg)
			continue
		}

		parsed := urlutil.ParseGitURL(canonicalURL)
		if !parsed.Valid || !parsed.GitHub {
			result = append(result, pkg)
			continue
		}

		pkg.Origin = canonicalURL

		if len(pkg.Copyright) == 0 || len(pkg.License) == 0 {
			status, repo, err := s.scm.GetRepositoryInfo(s.ctx, parsed.Owner, parsed.Repo)
			if err != nil {
				slog.Error("Failed to get repository info", "owner", parsed.Owner, "repo", parsed.Repo, "error", err)
				result = append(result, pkg)
				continue
			}
			if status == 200 && repo != nil {
				if len(pkg.Copyright) == 0 {
					pkg.Copyright = []string{repo.Owner.Login}
				}
				if len(pkg.License) == 0 && repo.License != nil {
					if repo.License.SPDXID == "NOASSERTION" {
						pkg.License = []string{}
					} else {
						pkg.License = []string{repo.License.SPDXID}
					}
				}
			} else {
				return nil, fmt.Errorf("failed to get repository information for %s/%s", parsed.Owner, parsed.Repo)
			}
		}

		result = append(result, pkg)
	}

	return result, nil
}
