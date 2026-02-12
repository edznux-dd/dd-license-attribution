// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"io/fs"
	"os"
)

// RealFileSystem implements port.FileSystem using the OS file system.
type RealFileSystem struct{}

func (f *RealFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *RealFileSystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (f *RealFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (f *RealFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f *RealFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (f *RealFileSystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		// Convert relative path back to absolute
		if path == "." {
			return fn(root, d, err)
		}
		return fn(root+"/"+path, d, err)
	})
}
