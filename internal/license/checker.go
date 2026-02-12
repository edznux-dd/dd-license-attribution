// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package license

import (
	"log/slog"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

// Checker checks for cautionary licenses in metadata.
type Checker struct {
	cautionaryLicenses []string
}

// NewChecker creates a Checker with the given cautionary license prefixes.
func NewChecker(cautionaryLicenses []string) *Checker {
	return &Checker{cautionaryLicenses: cautionaryLicenses}
}

// CheckCautionaryLicenses logs warnings for any cautionary licenses found.
func (c *Checker) CheckCautionaryLicenses(metas []metadata.Metadata) {
	for _, meta := range metas {
		if len(meta.License) == 0 {
			continue
		}
		for _, lic := range meta.License {
			if c.isCautionary(lic) {
				slog.Warn("Package has a cautionary license",
					"package", meta.Name,
					"license", lic,
				)
			}
		}
	}
}

func (c *Checker) isCautionary(licenseText string) bool {
	upper := strings.ToUpper(licenseText)
	for _, keyword := range c.cautionaryLicenses {
		if strings.HasPrefix(upper, strings.ToUpper(keyword)) {
			return true
		}
	}
	return false
}
