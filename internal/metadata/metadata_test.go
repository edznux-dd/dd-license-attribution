// SPDX-License-Identifier: Apache-2.0
package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMetadata_NilSlices(t *testing.T) {
	m := NewMetadata("name", "v1", "origin", "", nil, nil)
	assert.NotNil(t, m.License)
	assert.NotNil(t, m.Copyright)
	assert.Empty(t, m.License)
	assert.Empty(t, m.Copyright)
}

func TestNewMetadata_WithValues(t *testing.T) {
	m := NewMetadata("pkg", "1.0", "https://example.com", "/src", []string{"MIT"}, []string{"Datadog"})
	assert.Equal(t, "pkg", m.Name)
	assert.Equal(t, "1.0", m.Version)
	assert.Equal(t, "https://example.com", m.Origin)
	assert.Equal(t, "/src", m.LocalSrcPath)
	assert.Equal(t, []string{"MIT"}, m.License)
	assert.Equal(t, []string{"Datadog"}, m.Copyright)
}

func TestProjectScope(t *testing.T) {
	assert.Equal(t, ProjectScope(0), ScopeAll)
	assert.Equal(t, ProjectScope(1), ScopeOnlyRootProject)
	assert.Equal(t, ProjectScope(2), ScopeOnlyTransitiveDependencies)
}
