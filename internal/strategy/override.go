// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package strategy

import (
	"encoding/json"
	"fmt"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
)

// OverrideType defines the type of override to apply.
type OverrideType string

const (
	OverrideAdd     OverrideType = "add"
	OverrideRemove  OverrideType = "remove"
	OverrideReplace OverrideType = "replace"
)

// OverrideTargetField defines which metadata field to match.
type OverrideTargetField string

const (
	TargetOrigin    OverrideTargetField = "origin"
	TargetComponent OverrideTargetField = "component"
)

// OverrideRule defines a single override rule.
type OverrideRule struct {
	Type        OverrideType
	Target      map[OverrideTargetField]string
	Replacement *metadata.Metadata // nil for remove rules
}

// OverrideStrategy applies override rules to metadata.
type OverrideStrategy struct {
	rules       []OverrideRule
	unusedRules []OverrideRule
}

// NewOverrideStrategy creates an OverrideStrategy from the given rules.
func NewOverrideStrategy(rules []OverrideRule) *OverrideStrategy {
	unused := make([]OverrideRule, len(rules))
	copy(unused, rules)
	return &OverrideStrategy{
		rules:       rules,
		unusedRules: unused,
	}
}

// AugmentMetadata applies override rules to the metadata list.
func (s *OverrideStrategy) AugmentMetadata(metas []metadata.Metadata) ([]metadata.Metadata, error) {
	result := make([]metadata.Metadata, 0, len(metas))
	result = append(result, metas...)

	for _, meta := range metas {
		for _, rule := range s.rules {
			if !s.matchesTarget(meta, rule.Target) {
				continue
			}
			switch rule.Type {
			case OverrideAdd:
				if rule.Replacement == nil {
					return nil, fmt.Errorf("replacement for add should always be a dictionary")
				}
				result = append(result, *rule.Replacement)
				s.markUsed(rule)

			case OverrideRemove:
				result = removeMetadata(result, meta)
				s.markUsed(rule)

			case OverrideReplace:
				if rule.Replacement == nil {
					return nil, fmt.Errorf("replacement for replace should always be a dictionary")
				}
				result = removeMetadata(result, meta)
				result = append(result, *rule.Replacement)
				s.markUsed(rule)
			}
		}
	}

	return result, nil
}

// UnusedTargets returns the targets of rules that were not matched.
func (s *OverrideStrategy) UnusedTargets() []map[OverrideTargetField]string {
	targets := make([]map[OverrideTargetField]string, 0, len(s.unusedRules))
	for _, rule := range s.unusedRules {
		targets = append(targets, rule.Target)
	}
	return targets
}

func (s *OverrideStrategy) matchesTarget(meta metadata.Metadata, target map[OverrideTargetField]string) bool {
	if len(target) == 0 {
		return false
	}
	for field, value := range target {
		switch field {
		case TargetComponent:
			if meta.Name != value {
				return false
			}
		case TargetOrigin:
			if meta.Origin != value {
				return false
			}
		}
	}
	return true
}

func (s *OverrideStrategy) markUsed(rule OverrideRule) {
	for i, unused := range s.unusedRules {
		if targetsEqual(unused.Target, rule.Target) && unused.Type == rule.Type {
			s.unusedRules = append(s.unusedRules[:i], s.unusedRules[i+1:]...)
			return
		}
	}
}

func targetsEqual(a, b map[OverrideTargetField]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func removeMetadata(metas []metadata.Metadata, target metadata.Metadata) []metadata.Metadata {
	result := make([]metadata.Metadata, 0, len(metas))
	removed := false
	for _, m := range metas {
		if !removed && m.Name == target.Name && m.Origin == target.Origin {
			removed = true
			continue
		}
		result = append(result, m)
	}
	return result
}

// JSONToOverrideRules converts a JSON array to a list of OverrideRule objects.
func JSONToOverrideRules(data []byte) ([]OverrideRule, error) {
	var rawRules []json.RawMessage
	if err := json.Unmarshal(data, &rawRules); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	rules := make([]OverrideRule, 0, len(rawRules))
	for _, raw := range rawRules {
		var obj struct {
			OverrideType string                       `json:"override_type"`
			Target       map[string]string            `json:"target"`
			Replacement  *overrideReplacementJSON     `json:"replacement"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("invalid override rule: %w", err)
		}

		// Parse targets
		targets := make(map[OverrideTargetField]string)
		for key, val := range obj.Target {
			switch key {
			case "origin":
				targets[TargetOrigin] = val
			case "component":
				targets[TargetComponent] = val
			default:
				return nil, fmt.Errorf("target field must be origin or component: %s", key)
			}
		}

		// Validate based on override type
		overrideType := OverrideType(obj.OverrideType)
		switch overrideType {
		case OverrideAdd, OverrideReplace:
			if obj.Replacement == nil {
				return nil, fmt.Errorf("replacement must be a dictionary for add or replace rules")
			}
		case OverrideRemove:
			// replacement can be nil
		default:
			return nil, fmt.Errorf("invalid override type: %s", obj.OverrideType)
		}

		var replacement *metadata.Metadata
		if obj.Replacement != nil {
			lic := obj.Replacement.License
			if lic == nil {
				lic = []string{}
			}
			cr := obj.Replacement.Copyright
			if cr == nil {
				cr = []string{}
			}
			m := metadata.NewMetadata(
				obj.Replacement.Name,
				obj.Replacement.Version,
				obj.Replacement.Origin,
				"",
				lic,
				cr,
			)
			replacement = &m
		}

		rules = append(rules, OverrideRule{
			Type:        overrideType,
			Target:      targets,
			Replacement: replacement,
		})
	}
	return rules, nil
}

type overrideReplacementJSON struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Origin    string   `json:"origin"`
	License   []string `json:"license"`
	Copyright []string `json:"copyright"`
}
