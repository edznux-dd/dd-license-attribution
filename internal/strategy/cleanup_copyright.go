// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package strategy

import (
	"regexp"
	"sort"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

var copyrightCleanupRegex = regexp.MustCompile(
	`(?i)copyright(?:ed)?|\(c\)|\b\d{4}(?:\s?-\s?(?:\d{4}|present))?\b`,
)

// CleanupCopyrightStrategy cleans copyright strings by removing
// "copyright", "(c)", and year ranges.
type CleanupCopyrightStrategy struct{}

// AugmentMetadata cleans copyright fields in all metadata entries.
func (s *CleanupCopyrightStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	result := make([]metadata.Metadata, len(metas))
	for i, meta := range metas {
		result[i] = processMetadata(meta)
	}
	return result, nil
}

func processMetadata(meta metadata.Metadata) metadata.Metadata {
	if len(meta.Copyright) == 0 {
		return meta
	}

	seen := make(map[string]struct{})
	cleaned := make([]string, 0, len(meta.Copyright))

	for _, item := range meta.Copyright {
		if item == "" {
			continue
		}
		processed := removeUnnecessaryStrings(item)
		processed = collapseWhitespace(processed)
		if processed == "" {
			continue
		}
		if _, ok := seen[processed]; !ok {
			seen[processed] = struct{}{}
			cleaned = append(cleaned, processed)
		}
	}

	sort.Strings(cleaned)
	meta.Copyright = cleaned
	return meta
}

func removeUnnecessaryStrings(text string) string {
	return copyrightCleanupRegex.ReplaceAllString(text, "")
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
