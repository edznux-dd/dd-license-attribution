// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package port

import "context"

// ProcessRunner abstracts command execution.
type ProcessRunner interface {
	// Run executes a command and returns its exit code.
	Run(ctx context.Context, name string, args ...string) (int, error)
	// Output executes a command and returns its stdout.
	Output(ctx context.Context, name string, args ...string) (string, error)
	// OutputInDir executes a command in a specific directory and returns stdout.
	OutputInDir(ctx context.Context, dir string, name string, args ...string) (string, error)
	// RunInDir executes a command in a specific directory and returns its exit code.
	RunInDir(ctx context.Context, dir string, name string, args ...string) (int, error)
}
