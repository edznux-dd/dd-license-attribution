// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package collector

import (
	"strings"

	"github.com/DataDog/dd-license-attribution/internal/metadata"
	"github.com/DataDog/dd-license-attribution/internal/strategy"
)

// MetadataCollector runs strategies sequentially to collect metadata.
type MetadataCollector struct {
	strategies []strategy.Strategy
}

// New creates a MetadataCollector with the given strategies.
func New(strategies []strategy.Strategy) *MetadataCollector {
	return &MetadataCollector{strategies: strategies}
}

// CollectMetadata runs all strategies in sequence, starting from an initial
// metadata entry derived from the package URL.
func (c *MetadataCollector) CollectMetadata(pkg string) ([]metadata.Metadata, error) {
	name := strings.TrimPrefix(strings.TrimPrefix(pkg, "https://"), "http://")
	initial := metadata.NewMetadata(name, "", pkg, "", nil, nil)
	metas := []metadata.Metadata{initial}

	for _, s := range c.strategies {
		var err error
		metas, err = s.AugmentMetadata(metas)
		if err != nil {
			return nil, err
		}
	}
	return metas, nil
}
