// SPDX-License-Identifier: Apache-2.0
package cleaner

import (
	"context"
	"log/slog"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/strutil"
)

// ChangeInfo describes a single license change made by the cleaner.
type ChangeInfo struct {
	Row       int    `json:"row"`
	Component string `json:"component"`
	Origin    string `json:"origin"`
	Original  string `json:"original"`
	Converted string `json:"converted"`
}

// CleanResult contains the summary of a clean operation.
type CleanResult struct {
	TotalRows     int          `json:"total_rows"`
	ModifiedCount int          `json:"modified_count"`
	Changes       []ChangeInfo `json:"changes"`
}

// ChangeCallback is called for each proposed change. Return true to accept.
type ChangeCallback func(info ChangeInfo) bool

// SPDXCleaner converts long/non-standard license texts to SPDX identifiers using an LLM.
type SPDXCleaner struct {
	llm port.LLMClient
}

// NewSPDXCleaner creates a new SPDXCleaner.
func NewSPDXCleaner(llm port.LLMClient) *SPDXCleaner {
	return &SPDXCleaner{llm: llm}
}

// CleanMetadata processes metadata list, converting long license texts to SPDX IDs.
func (c *SPDXCleaner) CleanMetadata(ctx context.Context, metas []metadata.Metadata, callback ChangeCallback) (*CleanResult, []metadata.Metadata) {
	result := &CleanResult{
		TotalRows: len(metas),
		Changes:   []ChangeInfo{},
	}

	cleaned := make([]metadata.Metadata, len(metas))
	copy(cleaned, metas)

	for i := range cleaned {
		var newLicenses []string
		modified := false

		for _, lic := range cleaned[i].License {
			if !strutil.IsLongLicense(lic) {
				newLicenses = append(newLicenses, lic)
				continue
			}

			spdx, err := c.llm.ConvertToSPDX(ctx, lic)
			if err != nil {
				slog.Warn("LLM conversion failed", "component", cleaned[i].Name, "error", err)
				newLicenses = append(newLicenses, lic)
				continue
			}

			change := ChangeInfo{
				Row:       i,
				Component: cleaned[i].Name,
				Origin:    cleaned[i].Origin,
				Original:  lic,
				Converted: spdx,
			}

			if callback != nil {
				if !callback(change) {
					newLicenses = append(newLicenses, lic)
					continue
				}
			}

			newLicenses = append(newLicenses, spdx)
			result.Changes = append(result.Changes, change)
			modified = true
		}

		cleaned[i].License = newLicenses
		if modified {
			result.ModifiedCount++
		}
	}

	return result, cleaned
}
