// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package strutil

import "strings"

// IsLongLicense checks if a license text is a long description that should not
// be split or processed as a simple identifier.
func IsLongLicense(licenseText string) bool {
	return strings.Contains(licenseText, "\n") || len(licenseText) > 50
}

// CleanupLicenses splits licenses by " AND ", removes duplicates,
// and filters out known non-license identifiers.
func CleanupLicenses(licenses []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, lic := range licenses {
		parts := strings.Split(lic, " AND ")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if part == "LicenseRef-scancode-unknown-license-reference" {
				continue
			}
			if part == "LicenseRef-scancode-generic-cla" {
				continue
			}
			if _, ok := seen[part]; !ok {
				seen[part] = struct{}{}
				result = append(result, part)
			}
		}
	}
	return result
}
