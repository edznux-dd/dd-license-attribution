// SPDX-License-Identifier: Apache-2.0
package strategy

import (
	"testing"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicense3rdPartyStrategy_BasicCSV(t *testing.T) {
	csv := `"component","origin","license","copyright"
"pkg-a","https://github.com/org/pkg-a","['MIT']","['Org Inc']"
"pkg-b","https://github.com/org/pkg-b","['Apache-2.0']","['Other Corp']"
`
	fs := &testutil.MockFileSystem{
		Files: map[string][]byte{
			"/data/LICENSE-3rdparty.csv": []byte(csv),
		},
	}

	s := NewLicense3rdPartyStrategy("/data/LICENSE-3rdparty.csv", fs)
	result, err := s.AugmentMetadata(nil)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "pkg-a", result[0].Name)
	assert.Equal(t, "https://github.com/org/pkg-a", result[0].Origin)
	assert.Equal(t, []string{"MIT"}, result[0].License)
	assert.Equal(t, []string{"Org Inc"}, result[0].Copyright)
	assert.Equal(t, "pkg-b", result[1].Name)
}

func TestLicense3rdPartyStrategy_MergesExisting(t *testing.T) {
	csv := `"component","origin","license","copyright"
"pkg-a","https://github.com/org/pkg-a","['MIT']","['Org Inc']"
`
	fs := &testutil.MockFileSystem{
		Files: map[string][]byte{
			"/data/LICENSE-3rdparty.csv": []byte(csv),
		},
	}

	existing := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "1.0", "", "", nil, nil),
	}

	s := NewLicense3rdPartyStrategy("/data/LICENSE-3rdparty.csv", fs)
	result, err := s.AugmentMetadata(existing)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "pkg-a", result[0].Name)
	assert.Equal(t, "1.0", result[0].Version)
	assert.Equal(t, "https://github.com/org/pkg-a", result[0].Origin)
	assert.Equal(t, []string{"MIT"}, result[0].License)
	assert.Equal(t, []string{"Org Inc"}, result[0].Copyright)
}

func TestLicense3rdPartyStrategy_DoesNotOverwriteExisting(t *testing.T) {
	csv := `"component","origin","license","copyright"
"pkg-a","https://github.com/org/pkg-a","['MIT']","['Org Inc']"
`
	fs := &testutil.MockFileSystem{
		Files: map[string][]byte{
			"/data/LICENSE-3rdparty.csv": []byte(csv),
		},
	}

	existing := []metadata.Metadata{
		metadata.NewMetadata("pkg-a", "1.0", "https://original.com", "",
			[]string{"BSD-3-Clause"}, []string{"Original Author"}),
	}

	s := NewLicense3rdPartyStrategy("/data/LICENSE-3rdparty.csv", fs)
	result, err := s.AugmentMetadata(existing)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "https://original.com", result[0].Origin)
	assert.Equal(t, []string{"BSD-3-Clause"}, result[0].License)
	assert.Equal(t, []string{"Original Author"}, result[0].Copyright)
}

func TestLicense3rdPartyStrategy_MissingFile(t *testing.T) {
	fs := &testutil.MockFileSystem{Files: map[string][]byte{}}
	s := NewLicense3rdPartyStrategy("/data/missing.csv", fs)
	_, err := s.AugmentMetadata(nil)
	assert.Error(t, err)
}

func TestLicense3rdPartyStrategy_MissingColumns(t *testing.T) {
	csv := `"name","url"
"pkg-a","https://github.com/org/pkg-a"
`
	fs := &testutil.MockFileSystem{
		Files: map[string][]byte{
			"/data/bad.csv": []byte(csv),
		},
	}
	s := NewLicense3rdPartyStrategy("/data/bad.csv", fs)
	_, err := s.AugmentMetadata(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "component, origin, license, copyright")
}

func TestParsePythonList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", []string{}},
		{"empty list", "[]", []string{}},
		{"single item", "['MIT']", []string{"MIT"}},
		{"multiple items", "['MIT', 'BSD-3-Clause']", []string{"MIT", "BSD-3-Clause"}},
		{"double quotes", `["MIT", "Apache-2.0"]`, []string{"MIT", "Apache-2.0"}},
		{"no quotes", "[MIT]", []string{"MIT"}},
		{"spaces", "  ['MIT']  ", []string{"MIT"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePythonList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
