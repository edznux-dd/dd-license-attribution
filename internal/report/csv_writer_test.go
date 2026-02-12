// SPDX-License-Identifier: Apache-2.0
package report

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

func TestCSVWriter_BasicOutput(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "https://example.com/a", "", []string{"MIT"}, []string{"Datadog"}),
	}

	result := w.Write(metas)

	lines := strings.Split(result, "\r\n")
	assert.Equal(t, `"component","origin","license","copyright"`, lines[0])
	assert.Equal(t, `"pkg-a","https://example.com/a","['MIT']","['Datadog']"`, lines[1])
}

func TestCSVWriter_Deduplication(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "origin-a", "", []string{"MIT"}, []string{"Corp A"}),
		metadata.NewMetadata("pkg-a", "", "origin-a", "", []string{"BSD-3-Clause"}, []string{"Corp B"}),
	}

	result := w.Write(metas)

	lines := strings.Split(result, "\r\n")
	// Header + 1 data row + trailing empty from final \r\n
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[1], "'BSD-3-Clause'")
	assert.Contains(t, lines[1], "'MIT'")
	assert.Contains(t, lines[1], "'Corp A'")
	assert.Contains(t, lines[1], "'Corp B'")
}

func TestCSVWriter_SortedOutput(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("z-pkg", "", "origin-z", "", []string{"MIT"}, nil),
		metadata.NewMetadata("a-pkg", "", "origin-a", "", []string{"MIT"}, nil),
		metadata.NewMetadata("m-pkg", "", "origin-m", "", []string{"MIT"}, nil),
	}

	result := w.Write(metas)

	lines := strings.Split(result, "\r\n")
	assert.Contains(t, lines[1], "a-pkg")
	assert.Contains(t, lines[2], "m-pkg")
	assert.Contains(t, lines[3], "z-pkg")
}

func TestCSVWriter_EmptyLists(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg", "", "origin", "", nil, nil),
	}

	result := w.Write(metas)
	assert.Contains(t, result, `"[]","[]"`)
}

func TestCSVWriter_PythonListFormat(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg", "", "origin", "", []string{"Apache-2.0", "MIT"}, []string{"Datadog, Inc.", "Other Corp"}),
	}

	result := w.Write(metas)
	// Licenses should be sorted alphabetically within the list
	assert.Contains(t, result, "['Apache-2.0', 'MIT']")
	assert.Contains(t, result, "['Datadog, Inc.', 'Other Corp']")
}

func TestPythonRepr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MIT", "'MIT'"},
		{"no quotes", "'no quotes'"},
		{"it's here", `"it's here"`},
		{`has "double"`, `'has "double"'`},
		{`both 'single' and "double"`, `'both \'single\' and "double"'`},
		{"", "''"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, pythonRepr(tc.input))
		})
	}
}

func TestCSVWriter_PythonReprInList(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg", "", "origin", "", []string{"MIT"}, []string{"it's Corp", "Normal Corp"}),
	}

	result := w.Write(metas)
	// The string "it's Corp" contains a single quote, so Python repr wraps in double quotes
	// Inside the CSV (which uses double-quote escaping), it becomes ""it's Corp""
	assert.Contains(t, result, `""it's Corp""`)
	assert.Contains(t, result, "'Normal Corp'")
}

func TestCSVWriter_CRLFLineEndings(t *testing.T) {
	w := &CSVWriter{}
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg", "", "origin", "", []string{"MIT"}, nil),
	}

	result := w.Write(metas)
	assert.Contains(t, result, "\r\n")
	// Should not have bare \n without preceding \r
	for i, ch := range result {
		if ch == '\n' && (i == 0 || result[i-1] != '\r') {
			t.Error("Found bare \\n without \\r")
		}
	}
}
