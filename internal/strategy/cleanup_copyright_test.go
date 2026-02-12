// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

func TestCleanupCopyrightStrategy(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expected  []string
	}{
		{
			name:     "removes copyright and year",
			input:    []string{"Copyright 2023 Datadog, Inc."},
			expected: []string{"Datadog, Inc."},
		},
		{
			name:     "removes (c) and year range",
			input:    []string{"(c) 2020-2023 Some Company"},
			expected: []string{"Some Company"},
		},
		{
			name:     "removes copyrighted",
			input:    []string{"Copyrighted 2024 Foo Corp"},
			expected: []string{"Foo Corp"},
		},
		{
			name:     "removes year-present",
			input:    []string{"Copyright 2021-present Datadog"},
			expected: []string{"Datadog"},
		},
		{
			name:     "deduplicates and sorts",
			input:    []string{"Copyright 2023 B Corp", "Copyright 2024 A Corp", "(c) 2023 B Corp"},
			expected: []string{"A Corp", "B Corp"},
		},
		{
			name:     "empty copyright",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "removes empty after cleaning",
			input:    []string{"Copyright 2023"},
			expected: []string{},
		},
		{
			name:     "nil copyright stays nil-safe",
			input:    nil,
			expected: nil,
		},
	}

	s := &CleanupCopyrightStrategy{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []metadata.Metadata{
				{Name: "test", Copyright: tt.input},
			}
			result, err := s.AugmentMetadata(input)
			require.NoError(t, err)
			require.Len(t, result, 1)
			if tt.expected == nil {
				assert.Empty(t, result[0].Copyright)
			} else {
				assert.Equal(t, tt.expected, result[0].Copyright)
			}
		})
	}
}
