// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/go-enry/go-license-detector/v4/licensedb"
	"github.com/go-enry/go-license-detector/v4/licensedb/filer"

	"github.com/DataDog/dd-license-attribution/internal/port"
)

// RealLicenseDetector implements port.LicenseDetector using go-license-detector
// for license detection and regex-based copyright extraction.
type RealLicenseDetector struct{}

// DetectLicenses detects licenses in the given directory using go-license-detector.
func (d *RealLicenseDetector) DetectLicenses(dir string) ([]port.DetectedLicense, error) {
	f, err := filer.FromDirectory(dir)
	if err != nil {
		return nil, err
	}

	matches, err := licensedb.Detect(f)
	if err != nil {
		// ErrNoLicenseFound is not a fatal error
		if err == licensedb.ErrNoLicenseFound {
			return nil, nil
		}
		return nil, err
	}

	var result []port.DetectedLicense
	for id, match := range matches {
		result = append(result, port.DetectedLicense{
			Name:       id,
			Confidence: match.Confidence,
		})
	}
	return result, nil
}

// copyrightPattern matches common copyright statements in source files.
var copyrightPattern = regexp.MustCompile(
	`(?i)(?:copyright|©|\(c\))\s+(?:(?:\d{4}(?:\s*[-–,]\s*\d{4})*)\s+)?(.+?)(?:\.\s|$)`,
)

// holderPattern extracts a holder name from a copyright line.
var holderPattern = regexp.MustCompile(
	`(?i)(?:copyright|©|\(c\))\s+(?:\d{4}(?:\s*[-–,]\s*(?:\d{4}|present))*\s+)?(.+)`,
)

// DetectCopyrights detects copyright holders by scanning the given files
// with regex patterns matching common copyright statement formats.
func (d *RealLicenseDetector) DetectCopyrights(files []string) ([]port.DetectedCopyright, error) {
	var result []port.DetectedCopyright
	seen := make(map[string]struct{})

	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if m := holderPattern.FindStringSubmatch(line); m != nil {
				holder := strings.TrimSpace(m[1])
				// Clean up trailing punctuation
				holder = strings.TrimRight(holder, ".")
				holder = strings.TrimRight(holder, ",")
				holder = strings.TrimSpace(holder)
				if holder == "" {
					continue
				}
				if _, ok := seen[holder]; ok {
					continue
				}
				seen[holder] = struct{}{}
				result = append(result, port.DetectedCopyright{
					Holder: holder,
				})
			}
		}
		f.Close()
	}

	return result, nil
}
