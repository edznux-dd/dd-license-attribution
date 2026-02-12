// SPDX-License-Identifier: Apache-2.0
package license

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

func TestChecker_IsCautionary(t *testing.T) {
	checker := NewChecker([]string{"GPL", "EUPL", "AGPL"})

	tests := []struct {
		license  string
		expected bool
	}{
		{"GPL-2.0", true},
		{"GPL-3.0-only", true},
		{"AGPL-3.0", true},
		{"EUPL-1.2", true},
		{"MIT", false},
		{"Apache-2.0", false},
		{"BSD-3-Clause", false},
		{"gpl-2.0", true}, // case-insensitive
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			assert.Equal(t, tt.expected, checker.isCautionary(tt.license))
		})
	}
}

func TestChecker_CheckCautionaryLicenses_NoLicenses(t *testing.T) {
	checker := NewChecker([]string{"GPL"})
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg", "", "", "", nil, nil),
	}
	// Should not panic
	checker.CheckCautionaryLicenses(metas)
}
