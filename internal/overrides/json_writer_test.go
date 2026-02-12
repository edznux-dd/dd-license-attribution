// SPDX-License-Identifier: Apache-2.0
package overrides

import (
	"encoding/json"
	"testing"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON_Basic(t *testing.T) {
	replacement := metadata.NewMetadata("pkg-a", "", "https://github.com/org/pkg-a", "",
		[]string{"MIT"}, []string{"Author"})

	rules := []strategy.OverrideRule{
		{
			Type: strategy.OverrideReplace,
			Target: map[strategy.OverrideTargetField]string{
				strategy.TargetComponent: "pkg-a",
				strategy.TargetOrigin:    "https://old-url.com",
			},
			Replacement: &replacement,
		},
	}

	output, err := WriteJSON(rules)
	require.NoError(t, err)

	// Parse back to verify structure
	var parsed []jsonRule
	err = json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	assert.Equal(t, "replace", parsed[0].OverrideType)
	assert.Equal(t, "pkg-a", parsed[0].Target["component"])
	assert.Equal(t, "https://old-url.com", parsed[0].Target["origin"])
	assert.Equal(t, "pkg-a", parsed[0].Replacement.Name)
	assert.Equal(t, "https://github.com/org/pkg-a", parsed[0].Replacement.Origin)
	assert.Equal(t, []string{"MIT"}, parsed[0].Replacement.License)
	assert.Equal(t, []string{"Author"}, parsed[0].Replacement.Copyright)
}

func TestWriteJSON_RemoveRule(t *testing.T) {
	rules := []strategy.OverrideRule{
		{
			Type: strategy.OverrideRemove,
			Target: map[strategy.OverrideTargetField]string{
				strategy.TargetComponent: "pkg-to-remove",
			},
			Replacement: nil,
		},
	}

	output, err := WriteJSON(rules)
	require.NoError(t, err)

	var parsed []jsonRule
	err = json.Unmarshal([]byte(output), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	assert.Equal(t, "remove", parsed[0].OverrideType)
	assert.Nil(t, parsed[0].Replacement)
}

func TestWriteJSON_Empty(t *testing.T) {
	output, err := WriteJSON(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", output)
}

func TestGenerator_Generate(t *testing.T) {
	gen := NewGenerator()
	replacement := metadata.NewMetadata("pkg", "", "origin", "", []string{"MIT"}, nil)

	rules := []strategy.OverrideRule{
		{
			Type:        strategy.OverrideReplace,
			Target:      map[strategy.OverrideTargetField]string{strategy.TargetComponent: "pkg"},
			Replacement: &replacement,
		},
	}

	output, err := gen.Generate(rules)
	require.NoError(t, err)
	assert.Contains(t, output, "replace")
	assert.Contains(t, output, "pkg")
}
