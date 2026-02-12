// SPDX-License-Identifier: Apache-2.0
//
// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2024-present Datadog, Inc.

package port

import "io/fs"

// FileSystem abstracts file system operations.
type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	Exists(path string) bool
	MkdirAll(path string, perm fs.FileMode) error
	ReadDir(path string) ([]fs.DirEntry, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}
