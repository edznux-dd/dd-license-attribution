// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package port

import "context"

// LLMClient abstracts LLM API operations for license text conversion.
type LLMClient interface {
	// ConvertToSPDX converts a license text description to a valid SPDX expression.
	ConvertToSPDX(ctx context.Context, licenseText string) (string, error)
}
