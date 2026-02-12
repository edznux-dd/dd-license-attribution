// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

func TestOverrideStrategy_Replace(t *testing.T) {
	replacement := metadata.NewMetadata("replaced", "", "new-origin", "", []string{"MIT"}, []string{"New Corp"})
	rules := []OverrideRule{
		{
			Type:        OverrideReplace,
			Target:      map[OverrideTargetField]string{TargetComponent: "pkg-a"},
			Replacement: &replacement,
		},
	}

	s := NewOverrideStrategy(rules)
	input := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "origin-a", "", nil, nil),
		metadata.NewMetadata("pkg-b", "", "origin-b", "", nil, nil),
	}

	result, err := s.AugmentMetadata(input)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	// pkg-a should be replaced
	names := make(map[string]bool)
	for _, m := range result {
		names[m.Name] = true
	}
	assert.True(t, names["replaced"])
	assert.True(t, names["pkg-b"])
	assert.False(t, names["pkg-a"])
}

func TestOverrideStrategy_Remove(t *testing.T) {
	rules := []OverrideRule{
		{
			Type:   OverrideRemove,
			Target: map[OverrideTargetField]string{TargetOrigin: "origin-b"},
		},
	}

	s := NewOverrideStrategy(rules)
	input := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "origin-a", "", nil, nil),
		metadata.NewMetadata("pkg-b", "", "origin-b", "", nil, nil),
	}

	result, err := s.AugmentMetadata(input)
	require.NoError(t, err)

	assert.Len(t, result, 1)
	assert.Equal(t, "pkg-a", result[0].Name)
}

func TestOverrideStrategy_Add(t *testing.T) {
	added := metadata.NewMetadata("new-pkg", "", "new-origin", "", []string{"BSD-3-Clause"}, nil)
	rules := []OverrideRule{
		{
			Type:        OverrideAdd,
			Target:      map[OverrideTargetField]string{TargetComponent: "pkg-a"},
			Replacement: &added,
		},
	}

	s := NewOverrideStrategy(rules)
	input := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "origin-a", "", nil, nil),
	}

	result, err := s.AugmentMetadata(input)
	require.NoError(t, err)

	assert.Len(t, result, 2)
}

func TestOverrideStrategy_UnusedTargets(t *testing.T) {
	rules := []OverrideRule{
		{
			Type:   OverrideRemove,
			Target: map[OverrideTargetField]string{TargetComponent: "nonexistent"},
		},
	}

	s := NewOverrideStrategy(rules)
	input := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "origin-a", "", nil, nil),
	}

	_, err := s.AugmentMetadata(input)
	require.NoError(t, err)

	unused := s.UnusedTargets()
	assert.Len(t, unused, 1)
	assert.Equal(t, "nonexistent", unused[0][TargetComponent])
}

func TestOverrideStrategy_MultiFieldTarget(t *testing.T) {
	rules := []OverrideRule{
		{
			Type: OverrideRemove,
			Target: map[OverrideTargetField]string{
				TargetComponent: "pkg-a",
				TargetOrigin:    "origin-a",
			},
		},
	}

	s := NewOverrideStrategy(rules)
	input := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "", "origin-a", "", nil, nil),
		metadata.NewMetadata("pkg-a", "", "origin-b", "", nil, nil),
	}

	result, err := s.AugmentMetadata(input)
	require.NoError(t, err)

	// Only the one matching both fields should be removed
	assert.Len(t, result, 1)
	assert.Equal(t, "origin-b", result[0].Origin)
}

func TestJSONToOverrideRules(t *testing.T) {
	json := `[
		{
			"override_type": "replace",
			"target": {"component": "foo", "origin": "bar"},
			"replacement": {
				"name": "foo-new",
				"origin": "bar-new",
				"license": ["MIT"],
				"copyright": ["Datadog"]
			}
		},
		{
			"override_type": "remove",
			"target": {"component": "baz"},
			"replacement": null
		}
	]`

	rules, err := JSONToOverrideRules([]byte(json))
	require.NoError(t, err)
	assert.Len(t, rules, 2)

	assert.Equal(t, OverrideReplace, rules[0].Type)
	assert.Equal(t, "foo", rules[0].Target[TargetComponent])
	assert.Equal(t, "bar", rules[0].Target[TargetOrigin])
	assert.NotNil(t, rules[0].Replacement)
	assert.Equal(t, "foo-new", rules[0].Replacement.Name)
	assert.Equal(t, []string{"MIT"}, rules[0].Replacement.License)

	assert.Equal(t, OverrideRemove, rules[1].Type)
	assert.Nil(t, rules[1].Replacement)
}

func TestJSONToOverrideRules_InvalidType(t *testing.T) {
	json := `[{"override_type": "invalid", "target": {"component": "foo"}}]`
	_, err := JSONToOverrideRules([]byte(json))
	assert.Error(t, err)
}

func TestJSONToOverrideRules_AddWithoutReplacement(t *testing.T) {
	json := `[{"override_type": "add", "target": {"component": "foo"}, "replacement": null}]`
	_, err := JSONToOverrideRules([]byte(json))
	assert.Error(t, err)
}
