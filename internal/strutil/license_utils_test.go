// SPDX-License-Identifier: Apache-2.0
package strutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLongLicense(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"short SPDX", "MIT", false},
		{"medium SPDX", "BSD-3-Clause", false},
		{"long SPDX-like", "Apache-2.0", false},
		{"long text", "This is a very long license text that exceeds fifty characters definitely", true},
		{"multiline", "Line1\nLine2", true},
		{"exactly 50", "12345678901234567890123456789012345678901234567890", false},
		{"51 chars", "123456789012345678901234567890123456789012345678901", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsLongLicense(tt.input))
		})
	}
}

func TestCleanupLicenses(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "splits AND",
			input:    []string{"MIT AND BSD-3-Clause"},
			expected: []string{"MIT", "BSD-3-Clause"},
		},
		{
			name:     "removes unknown",
			input:    []string{"MIT", "LicenseRef-scancode-unknown-license-reference"},
			expected: []string{"MIT"},
		},
		{
			name:     "removes generic cla",
			input:    []string{"Apache-2.0", "LicenseRef-scancode-generic-cla"},
			expected: []string{"Apache-2.0"},
		},
		{
			name:     "deduplicates",
			input:    []string{"MIT", "MIT"},
			expected: []string{"MIT"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanupLicenses(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
