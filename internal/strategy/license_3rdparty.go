// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/port"
)

// License3rdPartyStrategy reads a LICENSE-3rdparty.csv and augments metadata.
type License3rdPartyStrategy struct {
	csvFilePath string
	fs          port.FileSystem
}

// NewLicense3rdPartyStrategy creates a new strategy.
func NewLicense3rdPartyStrategy(csvFilePath string, fs port.FileSystem) *License3rdPartyStrategy {
	return &License3rdPartyStrategy{csvFilePath: csvFilePath, fs: fs}
}

// AugmentMetadata reads the CSV file and merges entries into metadata.
func (s *License3rdPartyStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	data, err := s.fs.ReadFile(s.csvFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	r := csv.NewReader(strings.NewReader(string(data)))
	r.LazyQuotes = true

	// Read header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Build column mapping (case-insensitive)
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	requiredCols := []string{"component", "origin", "license", "copyright"}
	for _, col := range requiredCols {
		if _, ok := colMap[col]; !ok {
			return nil, fmt.Errorf("CSV file must contain columns: component, origin, license, copyright")
		}
	}

	componentIdx := colMap["component"]
	originIdx := colMap["origin"]
	licenseIdx := colMap["license"]
	copyrightIdx := colMap["copyright"]

	result := make([]metadata.Metadata, len(metas))
	copy(result, metas)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		component := record[componentIdx]
		origin := record[originIdx]
		licenseList := parsePythonList(record[licenseIdx])
		copyrightList := parsePythonList(record[copyrightIdx])

		// Check if entry already exists
		existing := findByName(result, component)
		if existing != nil {
			if existing.Origin == "" {
				existing.Origin = origin
			}
			if len(existing.License) == 0 {
				existing.License = licenseList
			}
			if len(existing.Copyright) == 0 {
				existing.Copyright = copyrightList
			}
		} else {
			result = append(result, metadata.NewMetadata(
				component, "", origin, "",
				licenseList, copyrightList,
			))
		}
	}

	return result, nil
}

func findByName(metas []metadata.Metadata, name string) *metadata.Metadata {
	for i := range metas {
		if metas[i].Name == name {
			return &metas[i]
		}
	}
	return nil
}

// parsePythonList parses a Python-style list string like "['MIT', 'BSD-3-Clause']".
func parsePythonList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return []string{}
	}

	// Remove brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	var result []string
	// Split by comma, handling quoted strings
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Remove surrounding quotes
		p = strings.Trim(p, "'\"")
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	if len(result) == 0 {
		return []string{}
	}
	return result
}
