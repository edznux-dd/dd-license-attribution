// SPDX-License-Identifier: Apache-2.0
package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/artifact"
	"github.com/DataDog/dd-license-attribution/internal/port"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

// LoadOverrideConfigs loads override rules from a JSON file.
func LoadOverrideConfigs(fs port.FileSystem, path string) ([]strategy.OverrideRule, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("override spec file not found: %s", path)
	}
	return strategy.JSONToOverrideRules(data)
}

// LoadMirrorConfigs loads mirror specifications from a JSON file.
func LoadMirrorConfigs(fs port.FileSystem, path string) ([]artifact.MirrorSpec, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mirror configuration file not found: %s", path)
	}

	var rawConfigs []struct {
		OriginalURL string            `json:"original_url"`
		MirrorURL   string            `json:"mirror_url"`
		RefMapping  map[string]string `json:"ref_mapping"`
	}
	if err := json.Unmarshal(data, &rawConfigs); err != nil {
		return nil, fmt.Errorf("invalid JSON in mirror configuration: %w", err)
	}

	mirrors := make([]artifact.MirrorSpec, 0, len(rawConfigs))
	for _, cfg := range rawConfigs {
		var refMapping map[[2]string][2]string
		if len(cfg.RefMapping) > 0 {
			var err error
			refMapping, err = parseRefMapping(cfg.RefMapping)
			if err != nil {
				return nil, err
			}
		}
		mirrors = append(mirrors, artifact.MirrorSpec{
			OriginalURL: cfg.OriginalURL,
			MirrorURL:   cfg.MirrorURL,
			RefMapping:  refMapping,
		})
	}
	return mirrors, nil
}

func parseRefMapping(raw map[string]string) (map[[2]string][2]string, error) {
	result := make(map[[2]string][2]string)
	for key, value := range raw {
		keyParts := strings.SplitN(key, ":", 2)
		if len(keyParts) != 2 {
			return nil, fmt.Errorf("invalid ref mapping key format: %s. Expected format: 'type:name'", key)
		}
		if err := validateRefType(keyParts[0]); err != nil {
			return nil, err
		}

		valParts := strings.SplitN(value, ":", 2)
		if len(valParts) != 2 {
			return nil, fmt.Errorf("invalid ref mapping value format: %s. Expected format: 'type:name'", value)
		}
		if err := validateRefType(valParts[0]); err != nil {
			return nil, err
		}

		result[[2]string{keyParts[0], keyParts[1]}] = [2]string{valParts[0], valParts[1]}
	}
	return result, nil
}

func validateRefType(t string) error {
	switch t {
	case "branch", "tag", "commit":
		return nil
	default:
		return fmt.Errorf("invalid ref type: %s. Valid types: branch, tag, commit", t)
	}
}
