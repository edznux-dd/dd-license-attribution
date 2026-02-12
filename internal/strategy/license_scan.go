// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"context"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/strutil"
)

// LicenseScanStrategy scans source code for license and copyright information.
type LicenseScanStrategy struct {
	scm                  *artifact.SourceCodeManager
	detector             port.LicenseDetector
	licenseSourceFiles   []string // lowercased, nil = scan all
	copyrightSourceFiles []string // lowercased, nil = scan all
	fsys                 port.FileSystem
	ctx                  context.Context
}

// NewLicenseScanStrategy creates a new license scan strategy.
func NewLicenseScanStrategy(
	ctx context.Context,
	scm *artifact.SourceCodeManager,
	detector port.LicenseDetector,
	fsys port.FileSystem,
	licenseSourceFiles []string,
	copyrightSourceFiles []string,
) *LicenseScanStrategy {
	var lowerLicense, lowerCopyright []string
	if licenseSourceFiles != nil {
		lowerLicense = make([]string, len(licenseSourceFiles))
		for i, f := range licenseSourceFiles {
			lowerLicense[i] = strings.ToLower(f)
		}
	}
	if copyrightSourceFiles != nil {
		lowerCopyright = make([]string, len(copyrightSourceFiles))
		for i, f := range copyrightSourceFiles {
			lowerCopyright[i] = strings.ToLower(f)
		}
	}
	return &LicenseScanStrategy{
		scm:                  scm,
		detector:             detector,
		licenseSourceFiles:   lowerLicense,
		copyrightSourceFiles: lowerCopyright,
		fsys:                 fsys,
		ctx:                  ctx,
	}
}

func (s *LicenseScanStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	var result []metadata.Metadata

	for _, pkg := range metas {
		if pkg.Origin == "" || (len(pkg.License) > 0 && len(pkg.Copyright) > 0) {
			result = append(result, pkg)
			continue
		}

		// Try local source path first
		if pkg.LocalSrcPath != "" {
			ref := &artifact.SourceCodeReference{
				RepoURL:       pkg.Origin,
				LocalRootPath: pkg.LocalSrcPath,
				LocalFullPath: pkg.LocalSrcPath,
			}
			if len(pkg.License) == 0 {
				pkg.License = s.getLicense(ref)
			}
			if len(pkg.Copyright) == 0 {
				pkg.Copyright = s.getCopyright(ref)
			}
		}

		// If still missing, clone
		if len(pkg.License) == 0 || len(pkg.Copyright) == 0 {
			ref, err := s.scm.GetCode(s.ctx, pkg.Origin, false)
			if err != nil || ref == nil {
				result = append(result, pkg)
				continue
			}

			if !s.fsys.Exists(ref.LocalFullPath) || !s.fsys.Exists(ref.LocalRootPath) {
				slog.Warn("Source path does not exist", "path", ref.LocalFullPath)
				result = append(result, pkg)
				continue
			}

			if len(pkg.License) == 0 {
				pkg.License = s.getLicense(ref)
			}
			if len(pkg.Copyright) == 0 {
				pkg.Copyright = s.getCopyright(ref)
			}
		}

		result = append(result, pkg)
	}

	return result, nil
}

func (s *LicenseScanStrategy) getLicense(ref *artifact.SourceCodeReference) []string {
	licenses, err := s.detector.DetectLicenses(ref.LocalFullPath)
	if err != nil {
		slog.Warn("License detection failed", "path", ref.LocalFullPath, "error", err)
		return nil
	}

	var result []string
	for _, lic := range licenses {
		if lic.Name != "" {
			result = append(result, lic.Name)
		}
	}

	if len(result) > 0 {
		return strutil.CleanupLicenses(result)
	}
	return nil
}

func (s *LicenseScanStrategy) getCopyright(ref *artifact.SourceCodeReference) []string {
	// Get candidate files
	var files []string
	s.fsys.WalkDir(ref.LocalFullPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.Contains(path, "/.git/") {
			return nil
		}
		if s.copyrightSourceFiles != nil {
			lower := strings.ToLower(d.Name())
			found := false
			for _, f := range s.copyrightSourceFiles {
				if lower == f {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})

	if len(files) == 0 {
		return nil
	}

	copyrights, err := s.detector.DetectCopyrights(files)
	if err != nil {
		return nil
	}

	// Priority: holders > authors > statements
	holders := make(map[string]struct{})
	authors := make(map[string]struct{})
	statements := make(map[string]struct{})

	for _, c := range copyrights {
		if c.Holder != "" {
			holders[c.Holder] = struct{}{}
		} else if c.Author != "" {
			authors[c.Author] = struct{}{}
		} else if c.Statement != "" {
			statements[c.Statement] = struct{}{}
		}
	}

	if len(holders) > 0 {
		return setToSlice(holders)
	}
	if len(authors) > 0 {
		return setToSlice(authors)
	}
	if len(statements) > 0 {
		return setToSlice(statements)
	}
	return nil
}

func setToSlice(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for k := range set {
		result = append(result, k)
	}
	return result
}
