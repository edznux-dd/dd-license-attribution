// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "dd-license-attribution",
	Short: "Collect license and copyright info for third-party dependencies",
	Long: `dd-license-attribution generates license attribution reports for open source
repositories by collecting metadata from GitHub SBOMs, package managers (Go, PyPI, npm),
and license file scanning.`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
