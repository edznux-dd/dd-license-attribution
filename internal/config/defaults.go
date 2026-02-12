// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package config

// PresetLicenseFileLocations defines common license file locations.
var PresetLicenseFileLocations = []string{
	"LICENSE",
	"LICENSE.code",
	"LICENSE.txt",
	"LICENSE.md",
	"COPYING",
	"LICENCE",
	"LICENCE.md",
	"license/LICENSE.txt",
}

// PresetCopyrightFileLocations defines common copyright file locations.
var PresetCopyrightFileLocations = []string{
	"NOTICE",
	"NOTICE.md",
	"NOTICE.txt",
	"AUTHORS",
	"AUTHORS.md",
	"AUTHORS.txt",
	"CONTRIBUTORS",
	"CONTRIBUTORS.md",
	"CONTRIBUTORS.txt",
	"LICENSE",
	"LICENSE.code",
	"LICENSE.txt",
	"LICENSE.md",
	"COPYING",
	"LICENCE",
	"LICENCE.md",
	"license/LICENSE.txt",
}

// PresetCautionaryLicenses lists licenses that should trigger warnings.
var PresetCautionaryLicenses = []string{
	"GPL",
	"EUPL",
	"AGPL",
}

// PresetCompanySuffixes lists company suffixes that should not cause splitting.
var PresetCompanySuffixes = []string{
	"inc",
	"inc.",
	"llc",
	"llc.",
}

// DefaultCacheTTL is the default cache time-to-live in seconds (24 hours).
const DefaultCacheTTL = 86400

