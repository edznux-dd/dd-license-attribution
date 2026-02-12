// SPDX-License-Identifier: Apache-2.0
package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

// mockStrategy is a test strategy that appends a metadata entry.
type mockStrategy struct {
	toAdd metadata.Metadata
}

func (m *mockStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	return append(metas, m.toAdd), nil
}

func TestMetadataCollector_BasicPipeline(t *testing.T) {
	strategies := []strategy.Strategy{
		&mockStrategy{toAdd: metadata.NewMetadata("dep-1", "1.0", "origin-1", "", []string{"MIT"}, nil)},
		&mockStrategy{toAdd: metadata.NewMetadata("dep-2", "2.0", "origin-2", "", []string{"BSD-3-Clause"}, nil)},
	}

	c := New(strategies)
	result, err := c.CollectMetadata("https://github.com/example/repo")
	require.NoError(t, err)

	// Initial metadata + 2 appended
	assert.Len(t, result, 3)
	assert.Equal(t, "github.com/example/repo", result[0].Name)
	assert.Equal(t, "dep-1", result[1].Name)
	assert.Equal(t, "dep-2", result[2].Name)
}

func TestMetadataCollector_StripProtocol(t *testing.T) {
	c := New(nil)
	result, err := c.CollectMetadata("https://github.com/foo/bar")
	require.NoError(t, err)
	assert.Equal(t, "github.com/foo/bar", result[0].Name)
}

func TestMetadataCollector_HTTPProtocol(t *testing.T) {
	c := New(nil)
	result, err := c.CollectMetadata("http://github.com/foo/bar")
	require.NoError(t, err)
	assert.Equal(t, "github.com/foo/bar", result[0].Name)
}

// errorStrategy returns an error.
type errorStrategy struct{}

func (e *errorStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	return nil, assert.AnError
}

func TestMetadataCollector_ErrorPropagation(t *testing.T) {
	strategies := []strategy.Strategy{&errorStrategy{}}
	c := New(strategies)
	_, err := c.CollectMetadata("https://example.com")
	assert.Error(t, err)
}
