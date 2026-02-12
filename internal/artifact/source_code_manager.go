// SPDX-License-Identifier: Apache-2.0
package artifact

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/urlutil"
)

// CacheTimestampFormat is the format used for cache directory timestamps.
const CacheTimestampFormat = "20060102_150405Z"

// NonAccessibleRepository is returned when a repository cannot be accessed.
type NonAccessibleRepository struct{ Msg string }

func (e *NonAccessibleRepository) Error() string { return e.Msg }

// UnauthorizedRepository is returned when the token lacks permissions.
type UnauthorizedRepository struct{ Msg string }

func (e *UnauthorizedRepository) Error() string { return e.Msg }

// RefType represents the type of git reference.
type RefType string

const (
	RefBranch RefType = "branch"
	RefTag    RefType = "tag"
	RefCommit RefType = "commit"
)

// MirrorSpec defines a repository mirror configuration.
type MirrorSpec struct {
	OriginalURL string
	MirrorURL   string
	RefMapping  map[[2]string][2]string // (refType, name) -> (refType, name)
}

// SourceCodeReference holds paths to cloned source code.
type SourceCodeReference struct {
	RepoURL       string
	Branch        string
	LocalRootPath string
	LocalFullPath string
}

// SourceCodeManager manages cloning and caching of source code.
type SourceCodeManager struct {
	cacheDir       string
	cacheTTL       int
	setupTime      time.Time
	timestampedDir string
	mirrors        []MirrorSpec
	gh             port.GitHubClient
	fs             port.FileSystem
	proc           port.ProcessRunner
	clock          port.Clock

	canonicalURLCache map[string][2]string           // url -> (canonical, apiURL)
	repoInfoCache     map[string]repoInfoCacheEntry
}

type repoInfoCacheEntry struct {
	status int
	repo   *port.GitHubRepository
}

// NewSourceCodeManager creates a SourceCodeManager.
func NewSourceCodeManager(
	cacheDir string,
	gh port.GitHubClient,
	fs port.FileSystem,
	proc port.ProcessRunner,
	clock port.Clock,
	cacheTTL int,
	mirrors []MirrorSpec,
) (*SourceCodeManager, error) {
	if !fs.Exists(cacheDir) {
		return nil, fmt.Errorf("local cache directory %s does not exist", cacheDir)
	}

	now := clock.Now()
	return &SourceCodeManager{
		cacheDir:          cacheDir,
		cacheTTL:          cacheTTL,
		setupTime:         now,
		timestampedDir:    now.Format(CacheTimestampFormat),
		mirrors:           mirrors,
		gh:                gh,
		fs:                fs,
		proc:              proc,
		clock:             clock,
		canonicalURLCache: make(map[string][2]string),
		repoInfoCache:     make(map[string]repoInfoCacheEntry),
	}, nil
}

// GetCanonicalURLs resolves a URL to its canonical GitHub URL and API URL.
func (m *SourceCodeManager) GetCanonicalURLs(ctx context.Context, url string) (string, string) {
	if cached, ok := m.canonicalURLCache[url]; ok {
		return cached[0], cached[1]
	}

	parsed := urlutil.ParseGitURL(url)
	if !parsed.Valid || !parsed.GitHub {
		result := [2]string{url, ""}
		m.canonicalURLCache[url] = result
		return url, ""
	}

	status, repo, _ := m.GetRepositoryInfo(ctx, parsed.Owner, parsed.Repo)
	if status == 200 && repo != nil {
		canonicalURL := repo.HTMLURL
		apiURL := repo.URL
		result := [2]string{canonicalURL, apiURL}
		m.canonicalURLCache[url] = result
		return canonicalURL, apiURL
	}

	fallbackURL := fmt.Sprintf("%s://%s/%s/%s", parsed.Protocol, parsed.Host, parsed.Owner, parsed.Repo)
	result := [2]string{fallbackURL, ""}
	m.canonicalURLCache[url] = result
	return fallbackURL, ""
}

// GetRepositoryInfo fetches repository info from GitHub with caching.
func (m *SourceCodeManager) GetRepositoryInfo(ctx context.Context, owner, repo string) (int, *port.GitHubRepository, error) {
	key := owner + "/" + repo
	if cached, ok := m.repoInfoCache[key]; ok {
		return cached.status, cached.repo, nil
	}

	status, repository, err := m.gh.GetRepository(ctx, owner, repo)
	if err != nil {
		return 0, nil, err
	}

	m.repoInfoCache[key] = repoInfoCacheEntry{status: status, repo: repository}
	return status, repository, nil
}

// GetCode clones or retrieves cached source code for a URL.
func (m *SourceCodeManager) GetCode(ctx context.Context, resourceURL string, forceUpdate bool) (*SourceCodeReference, error) {
	parsed := urlutil.ParseGitURL(resourceURL)
	if !parsed.Valid || !parsed.GitHub {
		return nil, nil
	}

	canonicalURL, apiURL := m.GetCanonicalURLs(ctx, resourceURL)
	if apiURL == "" {
		return nil, nil
	}

	canonParsed := urlutil.ParseGitURL(canonicalURL)
	if !canonParsed.Valid || !canonParsed.GitHub {
		return nil, nil
	}

	owner := canonParsed.Owner
	repo := canonParsed.Repo
	repositoryURL := canonicalURL

	branch := "default_branch"
	if parsed.Branch != "" {
		// Try to extract valid ref
		ref := m.extractRef(ctx, parsed.Branch, repositoryURL)
		if ref != "" {
			branch = ref
		}
	}

	path := ""
	if parsed.PathRaw != "" {
		if strings.HasPrefix(parsed.PathRaw, "/tree/") {
			path = strings.TrimPrefix(parsed.PathRaw, "/tree/"+branch)
		} else if strings.HasPrefix(parsed.PathRaw, "/blob/") {
			parts := strings.SplitN(strings.TrimPrefix(parsed.PathRaw, "/blob/"), "/", -1)
			if len(parts) > 1 {
				path = "/" + strings.Join(parts[1:], "/")
			}
		}
	}

	// Get mirror URL if available
	effectiveURL, _, effectiveBranch, resolvedBranch := m.getMirrorURLAndRef(ctx, repositoryURL, RefBranch, branch)
	branch = resolvedBranch

	// Check cache
	if !forceUpdate {
		ref := m.findCached(owner, repo, branch)
		if ref != nil {
			ref.LocalFullPath = ref.LocalRootPath + path
			return ref, nil
		}
	}

	// Clone
	localBranchPath := fmt.Sprintf("%s/%s/%s-%s/%s", m.cacheDir, m.timestampedDir, owner, repo, branch)
	if err := m.fs.MkdirAll(localBranchPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	slog.Debug("Cloning repository", "url", effectiveURL, "branch", effectiveBranch, "path", localBranchPath)
	_, err := m.proc.Run(ctx, "git", "clone", "-c", "advice.detachedHead=False",
		"--depth", "1", "--branch="+effectiveBranch, effectiveURL, localBranchPath)
	if err != nil {
		return nil, fmt.Errorf("failed to clone %s: %w", effectiveURL, err)
	}

	return &SourceCodeReference{
		RepoURL:       repositoryURL,
		Branch:        branch,
		LocalRootPath: localBranchPath,
		LocalFullPath: localBranchPath + path,
	}, nil
}

func (m *SourceCodeManager) extractRef(ctx context.Context, ref, url string) string {
	parts := strings.Split(ref, "/")
	for i := range parts {
		guess := strings.Join(parts[:i+1], "/")
		code, _ := m.proc.Run(ctx, "sh", "-c",
			fmt.Sprintf("git ls-remote %s %s | grep -q %s", url, guess, guess))
		if code == 0 {
			return guess
		}
	}
	if len(parts) > 0 {
		guess := parts[0]
		code, _ := m.proc.Run(ctx, "sh", "-c",
			fmt.Sprintf("git ls-remote %s | grep -q %s", url, guess))
		if code == 0 {
			return guess
		}
	}
	return ""
}

func (m *SourceCodeManager) discoverDefaultBranch(ctx context.Context, url string) (string, error) {
	out, err := m.proc.Output(ctx, "git", "ls-remote", "--symref", url, "HEAD")
	if err != nil {
		return "", &NonAccessibleRepository{Msg: fmt.Sprintf("could not discover default branch for %s", url)}
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return "", &NonAccessibleRepository{Msg: fmt.Sprintf("could not discover default branch for %s", url)}
	}
	return strings.TrimPrefix(fields[1], "refs/heads/"), nil
}

func (m *SourceCodeManager) getMirrorURLAndRef(ctx context.Context, originalURL string, refType RefType, refName string) (string, RefType, string, string) {
	if refName == "default_branch" {
		discovered, err := m.discoverDefaultBranch(ctx, originalURL)
		if err == nil {
			refName = discovered
		}
	}

	for _, mirror := range m.mirrors {
		if mirror.OriginalURL != originalURL {
			continue
		}
		mirrorURL := mirror.MirrorURL
		if refName == "default_branch" {
			discovered, err := m.discoverDefaultBranch(ctx, mirrorURL)
			if err != nil {
				continue
			}
			refName = discovered
		}
		if mirror.RefMapping != nil {
			key := [2]string{string(refType), refName}
			if mapped, ok := mirror.RefMapping[key]; ok {
				return mirrorURL, RefType(mapped[0]), mapped[1], refName
			}
		}
		return mirrorURL, refType, refName, refName
	}
	return originalURL, refType, refName, refName
}

func (m *SourceCodeManager) findCached(owner, repo, branch string) *SourceCodeReference {
	entries, err := m.fs.ReadDir(m.cacheDir)
	if err != nil {
		return nil
	}

	// Sort entries in reverse chronological order
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	for _, name := range names {
		ts, err := time.Parse(CacheTimestampFormat, name)
		if err != nil {
			continue
		}
		if m.setupTime.Sub(ts).Seconds() > float64(m.cacheTTL) {
			break
		}
		localBranchPath := fmt.Sprintf("%s/%s/%s-%s/%s", m.cacheDir, name, owner, repo, branch)
		if m.fs.Exists(localBranchPath) {
			return &SourceCodeReference{
				RepoURL:       fmt.Sprintf("https://github.com/%s/%s", owner, repo),
				Branch:        branch,
				LocalRootPath: localBranchPath,
				LocalFullPath: localBranchPath,
			}
		}
	}
	return nil
}
