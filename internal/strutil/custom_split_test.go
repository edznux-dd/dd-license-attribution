// SPDX-License-Identifier: Apache-2.0
package strutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomSplit_Basic(t *testing.T) {
	cs := NewCustomSplit(nil)
	result := cs.Split("a, b, c", ',')
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestCustomSplit_Empty(t *testing.T) {
	cs := NewCustomSplit(nil)
	result := cs.Split("", ',')
	assert.Nil(t, result)
}

func TestCustomSplit_Whitespace(t *testing.T) {
	cs := NewCustomSplit(nil)
	result := cs.Split("   ", ',')
	assert.Nil(t, result)
}

func TestCustomSplit_Quoted(t *testing.T) {
	cs := NewCustomSplit(nil)
	result := cs.Split(`"a, b", c`, ',')
	assert.Equal(t, []string{"a, b", "c"}, result)
}

func TestCustomSplit_ProtectedTerms(t *testing.T) {
	cs := NewCustomSplit([]string{"inc", "inc.", "llc", "llc."})

	// "Company, Inc." should not be split at the comma before "Inc."
	result := cs.Split("Company, Inc., Other Corp", ',')
	assert.Equal(t, []string{"Company, Inc.", "Other Corp"}, result)
}

func TestCustomSplit_ProtectedTerms_LLCCase(t *testing.T) {
	cs := NewCustomSplit([]string{"inc", "inc.", "llc", "llc."})

	result := cs.Split("Company, LLC, Other", ',')
	assert.Equal(t, []string{"Company, LLC", "Other"}, result)
}

func TestCustomSplit_NoProtectedTerms(t *testing.T) {
	cs := NewCustomSplit(nil)
	result := cs.Split("Company, Inc., Other", ',')
	assert.Equal(t, []string{"Company", "Inc.", "Other"}, result)
}

func TestCustomSplit_SingleItem(t *testing.T) {
	cs := NewCustomSplit(nil)
	result := cs.Split("single", ',')
	assert.Equal(t, []string{"single"}, result)
}
