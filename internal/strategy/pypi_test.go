// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslateNameGHtoPyPI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "my-package", "my-package"},
		{"github url", "https://github.com/org/repo", "com.github.org/repo"},
		{"http github url", "http://github.com/org/repo", "com.github.org/repo"},
		{"no github", "gitlab.com/org/repo", "gitlab.com/org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, translateNameGHtoPyPI(tt.input))
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected int
	}{
		{"empty", nil, 0},
		{"unique", []string{"a", "b", "c"}, 3},
		{"duplicates", []string{"a", "b", "a", "c", "b"}, 3},
		{"single", []string{"a"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueStrings(tt.input)
			assert.Len(t, result, tt.expected)
		})
	}
}
