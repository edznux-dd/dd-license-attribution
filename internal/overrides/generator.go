// SPDX-License-Identifier: Apache-2.0
package overrides

import (
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

// Generator orchestrates override rule generation and writing.
type Generator struct{}

// NewGenerator creates a new Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate converts override rules to a JSON string.
func (g *Generator) Generate(rules []strategy.OverrideRule) (string, error) {
	return WriteJSON(rules)
}
