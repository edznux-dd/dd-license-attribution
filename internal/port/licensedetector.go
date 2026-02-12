// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package port

// DetectedLicense represents a detected license in a file.
type DetectedLicense struct {
	Name       string
	Confidence float32
}

// DetectedCopyright represents detected copyright info.
type DetectedCopyright struct {
	Holder    string
	Author    string
	Statement string
}

// LicenseDetector abstracts license detection in source code.
type LicenseDetector interface {
	// DetectLicenses detects licenses in the given directory.
	DetectLicenses(dir string) ([]DetectedLicense, error)
	// DetectCopyrights detects copyright holders in the given files.
	DetectCopyrights(files []string) ([]DetectedCopyright, error)
}
