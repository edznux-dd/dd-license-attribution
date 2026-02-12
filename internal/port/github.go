// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package port

import "context"

// GitHubSBOM represents the SBOM response from GitHub's dependency graph API.
type GitHubSBOM struct {
	Packages []GitHubSBOMPackage `json:"packages"`
}

// GitHubSBOMPackage represents a package in the GitHub SBOM.
type GitHubSBOMPackage struct {
	SPDXID            string `json:"SPDXID"`
	Name              string `json:"name"`
	VersionInfo       string `json:"versionInfo"`
	DownloadLocation  string `json:"downloadLocation"`
	LicenseDeclared   string `json:"licenseDeclared"`
	LicenseConcluded  string `json:"licenseConcluded"`
	CopyrightText     string `json:"copyrightText"`
}

// GitHubRepository represents repository information from the GitHub API.
type GitHubRepository struct {
	HTMLURL       string              `json:"html_url"`
	URL           string              `json:"url"`
	DefaultBranch string              `json:"default_branch"`
	License       *GitHubLicense      `json:"license"`
	Owner         GitHubOwner         `json:"owner"`
}

// GitHubLicense represents license information from the GitHub API.
type GitHubLicense struct {
	SPDXID string `json:"spdx_id"`
}

// GitHubOwner represents the owner information from the GitHub API.
type GitHubOwner struct {
	Login string `json:"login"`
}

// GitHubClient abstracts GitHub API operations.
type GitHubClient interface {
	// GetSBOM retrieves the SBOM for a repository.
	GetSBOM(ctx context.Context, owner, repo string) (int, *GitHubSBOM, error)
	// GetRepository retrieves repository information.
	GetRepository(ctx context.Context, owner, repo string) (int, *GitHubRepository, error)
}
