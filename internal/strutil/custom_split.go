// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package strutil

import (
	"encoding/csv"
	"strings"
)

// CustomSplit parses a delimited string with support for quoted values
// and protected terms (company suffixes like "Inc." that appear after commas).
type CustomSplit struct {
	protectedTerms []string
}

// NewCustomSplit creates a CustomSplit with optional protected terms.
func NewCustomSplit(protectedTerms []string) *CustomSplit {
	return &CustomSplit{protectedTerms: protectedTerms}
}

// Split parses the input string by the given delimiter.
func (cs *CustomSplit) Split(input string, delimiter rune) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}

	// Parse using CSV reader for quote handling
	r := csv.NewReader(strings.NewReader(input))
	r.Comma = delimiter
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	record, err := r.Read()
	if err != nil {
		// Fallback to simple split
		record = simpleSplit(input, delimiter)
	}

	// Clean up: trim whitespace, remove empty entries
	parsed := make([]string, 0, len(record))
	for _, v := range record {
		v = strings.TrimSpace(v)
		if v != "" {
			parsed = append(parsed, v)
		}
	}

	if len(cs.protectedTerms) == 0 {
		return parsed
	}

	// Re-merge items that should be protected
	result := make([]string, 0, len(parsed))
	for _, value := range parsed {
		if len(result) > 0 && !cs.shouldSplit(value) {
			result[len(result)-1] += string(delimiter) + " " + value
		} else {
			result = append(result, value)
		}
	}

	return result
}

func (cs *CustomSplit) shouldSplit(text string) bool {
	lower := strings.ToLower(text)
	for _, term := range cs.protectedTerms {
		if strings.HasPrefix(lower, term) {
			return false
		}
	}
	return true
}

func simpleSplit(input string, delimiter rune) []string {
	parts := strings.Split(input, string(delimiter))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
