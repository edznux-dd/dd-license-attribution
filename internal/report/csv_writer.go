// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

// CSVWriter writes metadata as a CSV report matching the Python output format.
type CSVWriter struct{}

type rowKey struct {
	component string
	origin    string
}

type rowData struct {
	component string
	origin    string
	license   map[string]struct{}
	copyright map[string]struct{}
}

// Write produces a CSV string from metadata, matching the Python format exactly.
// Format: all fields quoted, \r\n line terminator, deduplication by (name, origin),
// license/copyright formatted as Python-style sorted list strings.
func (w *CSVWriter) Write(metas []metadata.Metadata) string {
	// Combine metadata with same (name, origin)
	combined := make(map[rowKey]*rowData)
	var keys []rowKey

	for _, md := range metas {
		k := rowKey{component: md.Name, origin: md.Origin}
		if existing, ok := combined[k]; ok {
			for _, l := range md.License {
				existing.license[l] = struct{}{}
			}
			for _, c := range md.Copyright {
				existing.copyright[c] = struct{}{}
			}
		} else {
			rd := &rowData{
				component: md.Name,
				origin:    md.Origin,
				license:   make(map[string]struct{}),
				copyright: make(map[string]struct{}),
			}
			for _, l := range md.License {
				rd.license[l] = struct{}{}
			}
			for _, c := range md.Copyright {
				rd.copyright[c] = struct{}{}
			}
			combined[k] = rd
			keys = append(keys, k)
		}
	}

	// Sort by (component, origin)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].component != keys[j].component {
			return keys[i].component < keys[j].component
		}
		return keys[i].origin < keys[j].origin
	})

	var sb strings.Builder

	// Write header
	sb.WriteString("\"component\",\"origin\",\"license\",\"copyright\"\r\n")

	// Write rows
	for _, k := range keys {
		rd := combined[k]
		licenseStr := formatPythonList(rd.license)
		copyrightStr := formatPythonList(rd.copyright)

		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s\r\n",
			quoteCSV(rd.component),
			quoteCSV(rd.origin),
			quoteCSV(licenseStr),
			quoteCSV(copyrightStr),
		))
	}

	return sb.String()
}

// formatPythonList formats a set of strings as a Python-style sorted list string.
// e.g. ['Apache-2.0'] or ['BSD-3-Clause', 'MIT']
func formatPythonList(set map[string]struct{}) string {
	items := make([]string, 0, len(set))
	for k := range set {
		items = append(items, k)
	}
	sort.Strings(items)

	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = pythonRepr(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// pythonRepr mimics Python's repr() quoting rules for strings:
//   - No single quote in string: wrap in '...'
//   - Contains ' but not ": wrap in "..."
//   - Contains both ' and ": wrap in '...' with \' escaping
func pythonRepr(s string) string {
	hasSingle := strings.Contains(s, "'")
	hasDouble := strings.Contains(s, "\"")

	if hasSingle && !hasDouble {
		return "\"" + s + "\""
	}
	if hasSingle && hasDouble {
		return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
	}
	return "'" + s + "'"
}

// quoteCSV wraps a value in double quotes, escaping internal double quotes.
func quoteCSV(s string) string {
	escaped := strings.ReplaceAll(s, "\"", "\"\"")
	return "\"" + escaped + "\""
}
