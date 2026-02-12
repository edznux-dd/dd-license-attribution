// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package port

// UserPrompter abstracts user interaction via stdin/stdout.
type UserPrompter interface {
	// Confirm prompts the user with a yes/no question.
	Confirm(msg string) (bool, error)
	// Prompt prompts the user for text input with a default value.
	Prompt(msg string, defaultVal string) (string, error)
}
