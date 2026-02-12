// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/urlutil"
)

// NpmStrategy collects metadata from npm/yarn for Node.js dependencies.
type NpmStrategy struct {
	originalTopPackage string
	topPackage         string
	scm                *artifact.SourceCodeManager
	onlyRootProject    bool
	onlyTransitive     bool
	yarnSubdirs        []string
	fs                 port.FileSystem
	proc               port.ProcessRunner
	http               port.HTTPFetcher
	ctx                context.Context
}

// NewNpmStrategy creates a new npm strategy.
func NewNpmStrategy(
	ctx context.Context,
	topPackage string,
	scm *artifact.SourceCodeManager,
	scope metadata.ProjectScope,
	yarnSubdirs []string,
	fsys port.FileSystem,
	proc port.ProcessRunner,
	http port.HTTPFetcher,
) *NpmStrategy {
	originalTopPackage := topPackage
	canonical, _ := scm.GetCanonicalURLs(ctx, topPackage)
	if canonical != "" {
		topPackage = canonical
	}
	return &NpmStrategy{
		originalTopPackage: originalTopPackage,
		topPackage:         topPackage,
		scm:                scm,
		onlyRootProject:    scope == metadata.ScopeOnlyRootProject,
		onlyTransitive:     scope == metadata.ScopeOnlyTransitiveDependencies,
		yarnSubdirs:        yarnSubdirs,
		fs:                 fsys,
		proc:               proc,
		http:               http,
		ctx:                ctx,
	}
}

func (s *NpmStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	result := make([]metadata.Metadata, len(metas))
	copy(result, metas)

	ref, err := s.scm.GetCode(s.ctx, s.topPackage, false)
	if err != nil || ref == nil {
		return result, nil
	}

	projectPath := ref.LocalFullPath
	pkgJSONPath := projectPath + "/package.json"
	if !s.fs.Exists(pkgJSONPath) {
		return result, nil
	}

	pkgJSONData, err := s.fs.ReadFile(pkgJSONPath)
	if err != nil {
		return result, nil
	}

	var pkgJSON map[string]json.RawMessage
	if err := json.Unmarshal(pkgJSONData, &pkgJSON); err != nil {
		return result, nil
	}

	if _, ok := pkgJSON["workspaces"]; ok {
		slog.Warn("Node projects using workspaces are not supported yet")
		return result, nil
	}

	// Enrich root package from package.json
	s.enrichRootFromPackageJSON(pkgJSONData, result)

	if s.onlyRootProject {
		return result, nil
	}

	// Detect package manager and get dependencies
	var allDeps map[string]string

	if s.fs.Exists(projectPath + "/yarn.lock") {
		allDeps = s.getYarnDeps(projectPath)
	} else {
		allDeps = s.getNpmDeps(projectPath)
	}

	if len(allDeps) == 0 {
		return result, nil
	}

	slog.Info("Found packages", "count", len(allDeps))
	return s.enrichWithNpmRegistry(result, allDeps), nil
}

func (s *NpmStrategy) enrichRootFromPackageJSON(data []byte, metas []metadata.Metadata) {
	var pkgJSON struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		License interface{} `json:"license"`
		Author  interface{} `json:"author"`
	}
	if err := json.Unmarshal(data, &pkgJSON); err != nil {
		return
	}

	license := extractLicenseFromPkgData(pkgJSON.License)
	copyright := extractCopyrightFromAuthor(pkgJSON.Author)

	for i := range metas {
		isRoot := metas[i].Origin != "" &&
			(strings.Contains(metas[i].Origin, s.topPackage) ||
				strings.Contains(metas[i].Origin, s.originalTopPackage))
		if isRoot {
			if len(license) > 0 {
				metas[i].License = license
			}
			if len(copyright) > 0 {
				metas[i].Copyright = copyright
			}
			if pkgJSON.Version != "" {
				metas[i].Version = pkgJSON.Version
			}
			if pkgJSON.Name != "" {
				metas[i].Name = pkgJSON.Name
			}
			break
		}
	}
}

func (s *NpmStrategy) getYarnDeps(projectPath string) map[string]string {
	allDeps := make(map[string]string)

	out, err := s.proc.OutputInDir(s.ctx, projectPath, "yarn", "list", "--production", "--json", "--non-interactive")
	if err != nil || out == "" {
		return allDeps
	}

	// Extract aliases from yarn.lock
	aliases := s.extractAliasesFromYarnLock(projectPath)

	// Parse yarn list output
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var data struct {
			Type string `json:"type"`
			Data struct {
				Trees []struct {
					Name string `json:"name"`
				} `json:"trees"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}
		if data.Type != "tree" {
			continue
		}
		for _, tree := range data.Data.Trees {
			name := tree.Name
			if name == "" {
				continue
			}
			var pkgName, version string
			if strings.HasPrefix(name, "@") {
				parts := strings.SplitN(name[1:], "@", 2)
				if len(parts) == 2 {
					pkgName = "@" + parts[0]
					version = parts[1]
				}
			} else if strings.Contains(name, "@") {
				parts := strings.SplitN(name, "@", 2)
				pkgName = parts[0]
				version = parts[1]
			} else {
				pkgName = name
			}
			// Resolve aliases
			if real, ok := aliases[pkgName]; ok {
				pkgName = real
			}
			allDeps[pkgName] = version
		}
	}

	return allDeps
}

var yarnAliasPattern = regexp.MustCompile(`^"([^"]+)@npm:(@[^@]+|[^@]+)@[^"]+":$`)

func (s *NpmStrategy) extractAliasesFromYarnLock(projectPath string) map[string]string {
	aliases := make(map[string]string)
	data, err := s.fs.ReadFile(projectPath + "/yarn.lock")
	if err != nil {
		return aliases
	}

	for _, line := range strings.Split(string(data), "\n") {
		if m := yarnAliasPattern.FindStringSubmatch(line); m != nil {
			aliases[m[1]] = m[2]
		}
	}
	return aliases
}

func (s *NpmStrategy) getNpmDeps(projectPath string) map[string]string {
	// Run npm install --package-lock-only
	_, _ = s.proc.OutputInDir(s.ctx, projectPath, "npm", "install", "--package-lock-only", "--force")

	lockPath := projectPath + "/package-lock.json"
	if !s.fs.Exists(lockPath) {
		return nil
	}

	data, err := s.fs.ReadFile(lockPath)
	if err != nil {
		return nil
	}

	var lockData map[string]json.RawMessage
	if err := json.Unmarshal(data, &lockData); err != nil {
		return nil
	}

	packagesRaw, ok := lockData["packages"]
	if !ok {
		return nil
	}

	var packages map[string]json.RawMessage
	if err := json.Unmarshal(packagesRaw, &packages); err != nil {
		return nil
	}

	// Find root
	rootKey := ""
	for _, key := range []string{"", "./"} {
		if _, ok := packages[key]; ok {
			rootKey = key
			break
		}
	}
	if rootKey == "" && len(packages) == 0 {
		return nil
	}

	allDeps := make(map[string]string)
	aliases := s.extractAliasesFromPackageLock(data, projectPath)

	// Get root dependencies
	var rootPkg struct {
		Dependencies map[string]interface{} `json:"dependencies"`
	}
	if rootRaw, ok := packages[rootKey]; ok {
		json.Unmarshal(rootRaw, &rootPkg)
	}

	for depName := range rootPkg.Dependencies {
		realName := depName
		if alias, ok := aliases[depName]; ok {
			realName = alias
		}
		nodeKey := "node_modules/" + depName
		if pkgRaw, ok := packages[nodeKey]; ok {
			var pkg struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(pkgRaw, &pkg) == nil {
				allDeps[realName] = pkg.Version
			}
		}
	}

	// Extract transitive deps
	s.extractTransitiveDeps(packages, allDeps, aliases)

	return allDeps
}

func (s *NpmStrategy) extractAliasesFromPackageLock(data []byte, projectPath string) map[string]string {
	aliases := make(map[string]string)

	var lockData struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(data, &lockData); err != nil {
		return aliases
	}

	// Check root package dependencies for npm: aliases
	if rootRaw, ok := lockData.Packages[""]; ok {
		var rootPkg struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if json.Unmarshal(rootRaw, &rootPkg) == nil {
			for depName, depSpec := range rootPkg.Dependencies {
				if strings.HasPrefix(depSpec, "npm:") {
					realSpec := depSpec[4:]
					var realName string
					if strings.HasPrefix(realSpec, "@") {
						parts := strings.SplitN(realSpec, "@", 3)
						if len(parts) >= 3 {
							realName = "@" + parts[1]
						}
					} else {
						parts := strings.SplitN(realSpec, "@", 2)
						if len(parts) >= 2 {
							realName = parts[0]
						}
					}
					if realName != "" {
						aliases[depName] = realName
					}
				}
			}
		}
	}

	// Check node_modules entries for name mismatches
	for key, pkgRaw := range lockData.Packages {
		if !strings.HasPrefix(key, "node_modules/") {
			continue
		}
		pkgPath := strings.TrimPrefix(key, "node_modules/")
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(pkgRaw, &pkg) == nil && pkg.Name != "" && pkg.Name != pkgPath {
			aliases[pkgPath] = pkg.Name
		}
	}

	return aliases
}

func (s *NpmStrategy) extractTransitiveDeps(packages map[string]json.RawMessage, allDeps map[string]string, aliases map[string]string) {
	processed := make(map[string]bool)
	changed := true
	for changed {
		changed = false
		for pkgName := range allDeps {
			if processed[pkgName] {
				continue
			}
			nodeKey := "node_modules/" + pkgName
			if pkgRaw, ok := packages[nodeKey]; ok {
				var pkg struct {
					Dependencies map[string]interface{} `json:"dependencies"`
				}
				if json.Unmarshal(pkgRaw, &pkg) == nil {
					for depName := range pkg.Dependencies {
						realName := depName
						if alias, ok := aliases[depName]; ok {
							realName = alias
						}
						if _, exists := allDeps[realName]; !exists {
							depKey := "node_modules/" + depName
							if depRaw, ok := packages[depKey]; ok {
								var depPkg struct {
									Version string `json:"version"`
								}
								if json.Unmarshal(depRaw, &depPkg) == nil {
									allDeps[realName] = depPkg.Version
									changed = true
								}
							}
						}
					}
				}
			}
			processed[pkgName] = true
		}
	}
}

func (s *NpmStrategy) enrichWithNpmRegistry(metas []metadata.Metadata, deps map[string]string) []metadata.Metadata {
	result := make([]metadata.Metadata, len(metas))
	copy(result, metas)

	if s.onlyTransitive {
		filtered := make([]metadata.Metadata, 0)
		for _, m := range result {
			if m.Name != s.topPackage {
				filtered = append(filtered, m)
			}
		}
		result = filtered
	}

	for depName, version := range deps {
		license, copyright, pkgData := s.fetchNpmRegistryMetadata(depName, version)
		origin := s.determineOrigin(pkgData, depName)

		found := false
		for i := range result {
			if result[i].Name == depName && result[i].Version == version {
				found = true
				if (result[i].Origin == "" || !urlutil.IsValidGitURL(result[i].Origin)) && origin != "" {
					result[i].Origin = origin
				}
				if len(result[i].License) == 0 && len(license) > 0 {
					result[i].License = license
				}
				if len(result[i].Copyright) == 0 && len(copyright) > 0 {
					result[i].Copyright = copyright
				}
				break
			}
		}
		if !found {
			result = append(result, metadata.NewMetadata(
				depName, version, origin, "",
				license, copyright,
			))
		}
	}
	return result
}

func (s *NpmStrategy) fetchNpmRegistryMetadata(depName, version string) ([]string, []string, map[string]interface{}) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s/%s", depName, version)
	resp, err := s.http.Get(s.ctx, url)
	if err != nil || resp.StatusCode != 200 {
		return nil, nil, nil
	}

	var pkgData map[string]interface{}
	if err := json.Unmarshal(resp.Body, &pkgData); err != nil {
		return nil, nil, nil
	}

	license := extractLicenseFromPkgData(pkgData["license"])
	copyright := extractCopyrightFromAuthor(pkgData["author"])

	return license, copyright, pkgData
}

func (s *NpmStrategy) determineOrigin(pkgData map[string]interface{}, depName string) string {
	if pkgData == nil {
		return "npm:" + depName
	}

	if repo, ok := pkgData["repository"]; ok && repo != nil {
		switch r := repo.(type) {
		case map[string]interface{}:
			if url, ok := r["url"].(string); ok && url != "" {
				return url
			}
		case string:
			return r
		}
	}

	if homepage, ok := pkgData["homepage"].(string); ok && homepage != "" {
		return homepage
	}

	return "npm:" + depName
}

func extractLicenseFromPkgData(license interface{}) []string {
	if license == nil {
		return nil
	}
	switch l := license.(type) {
	case string:
		if l != "" {
			return []string{l}
		}
	case map[string]interface{}:
		if t, ok := l["type"].(string); ok && t != "" {
			return []string{t}
		}
	}
	return nil
}

func extractCopyrightFromAuthor(author interface{}) []string {
	if author == nil {
		return nil
	}
	switch a := author.(type) {
	case string:
		if a != "" {
			return []string{a}
		}
	case map[string]interface{}:
		if name, ok := a["name"].(string); ok && name != "" {
			return []string{name}
		}
	}
	return nil
}
