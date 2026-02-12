// SPDX-License-Identifier: Apache-2.0
package overrides

import (
	"encoding/json"

	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

type jsonRule struct {
	OverrideType string            `json:"override_type"`
	Target       map[string]string `json:"target"`
	Replacement  *jsonReplacement  `json:"replacement"`
}

type jsonReplacement struct {
	Name      string   `json:"name"`
	Origin    string   `json:"origin"`
	License   []string `json:"license"`
	Copyright []string `json:"copyright"`
}

// WriteJSON converts override rules to a JSON string.
func WriteJSON(rules []strategy.OverrideRule) (string, error) {
	jsonRules := make([]jsonRule, 0, len(rules))
	for _, rule := range rules {
		target := make(map[string]string)
		for field, value := range rule.Target {
			target[string(field)] = value
		}

		jr := jsonRule{
			OverrideType: string(rule.Type),
			Target:       target,
		}
		if rule.Replacement != nil {
			jr.Replacement = &jsonReplacement{
				Name:      rule.Replacement.Name,
				Origin:    rule.Replacement.Origin,
				License:   rule.Replacement.License,
				Copyright: rule.Replacement.Copyright,
			}
		}
		jsonRules = append(jsonRules, jr)
	}

	data, err := json.MarshalIndent(jsonRules, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
