// SPDX-License-Identifier: Apache-2.0
package cleaner

import (
	"context"
	"testing"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPDXCleaner_ConvertsLongLicenses(t *testing.T) {
	llm := &testutil.MockLLMClient{
		Responses: map[string]string{
			"MIT License\nPermission is hereby granted...": "MIT",
		},
	}

	c := NewSPDXCleaner(llm)
	ctx := context.Background()

	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "1.0", "https://example.com", "",
			[]string{"MIT License\nPermission is hereby granted..."}, []string{"Author"}),
	}

	result, cleaned := c.CleanMetadata(ctx, metas, nil)

	require.Len(t, cleaned, 1)
	assert.Equal(t, []string{"MIT"}, cleaned[0].License)
	assert.Equal(t, 1, result.TotalRows)
	assert.Equal(t, 1, result.ModifiedCount)
	assert.Len(t, result.Changes, 1)
	assert.Equal(t, "MIT License\nPermission is hereby granted...", result.Changes[0].Original)
	assert.Equal(t, "MIT", result.Changes[0].Converted)
}

func TestSPDXCleaner_SkipsShortLicenses(t *testing.T) {
	llm := &testutil.MockLLMClient{
		Responses: map[string]string{},
	}

	c := NewSPDXCleaner(llm)
	ctx := context.Background()

	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "1.0", "https://example.com", "",
			[]string{"MIT"}, []string{"Author"}),
	}

	result, cleaned := c.CleanMetadata(ctx, metas, nil)

	assert.Equal(t, []string{"MIT"}, cleaned[0].License)
	assert.Equal(t, 0, result.ModifiedCount)
}

func TestSPDXCleaner_CallbackCanReject(t *testing.T) {
	llm := &testutil.MockLLMClient{
		Responses: map[string]string{
			"Very long license text that spans multiple lines\nand has lots of content": "Apache-2.0",
		},
	}

	c := NewSPDXCleaner(llm)
	ctx := context.Background()

	originalLic := "Very long license text that spans multiple lines\nand has lots of content"
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "1.0", "https://example.com", "",
			[]string{originalLic}, []string{"Author"}),
	}

	// Callback rejects the change
	callback := func(info ChangeInfo) bool {
		return false
	}

	result, cleaned := c.CleanMetadata(ctx, metas, callback)

	assert.Equal(t, []string{originalLic}, cleaned[0].License)
	assert.Equal(t, 0, result.ModifiedCount)
	assert.Len(t, result.Changes, 0)
}

func TestSPDXCleaner_CallbackCanAccept(t *testing.T) {
	llm := &testutil.MockLLMClient{
		Responses: map[string]string{
			"Very long license text that spans multiple lines\nand has lots of content": "BSD-3-Clause",
		},
	}

	c := NewSPDXCleaner(llm)
	ctx := context.Background()

	originalLic := "Very long license text that spans multiple lines\nand has lots of content"
	metas := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "1.0", "https://example.com", "",
			[]string{originalLic}, []string{"Author"}),
	}

	callback := func(info ChangeInfo) bool {
		return true
	}

	result, cleaned := c.CleanMetadata(ctx, metas, callback)

	assert.Equal(t, []string{"BSD-3-Clause"}, cleaned[0].License)
	assert.Equal(t, 1, result.ModifiedCount)
}
