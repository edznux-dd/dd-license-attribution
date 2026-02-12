// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package metadata

// ProjectScope defines which packages to include in the analysis.
type ProjectScope int

const (
	ScopeAll ProjectScope = iota
	ScopeOnlyRootProject
	ScopeOnlyTransitiveDependencies
)

// Metadata stores metadata about a package.
type Metadata struct {
	Name         string   // component name (empty = unset)
	Version      string   // package version or commit hash
	Origin       string   // package manager purl or repository URL
	LocalSrcPath string   // local path to source code
	License      []string // SPDX format licenses (never nil, use empty slice)
	Copyright    []string // copyright holders (never nil, use empty slice)
}

// NewMetadata creates a Metadata with non-nil slices.
func NewMetadata(name, version, origin, localSrcPath string, license, copyright []string) Metadata {
	if license == nil {
		license = []string{}
	}
	if copyright == nil {
		copyright = []string{}
	}
	return Metadata{
		Name:         name,
		Version:      version,
		Origin:       origin,
		LocalSrcPath: localSrcPath,
		License:      license,
		Copyright:    copyright,
	}
}
