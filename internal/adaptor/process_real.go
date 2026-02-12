// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"bytes"
	"context"
	"os/exec"
)

// RealProcessRunner implements port.ProcessRunner using os/exec.
type RealProcessRunner struct{}

func (r *RealProcessRunner) Run(ctx context.Context, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = nil
	cmd.Stdout = nil
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), err
		}
		return 1, err
	}
	return 0, nil
}

func (r *RealProcessRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.String(), err
}

func (r *RealProcessRunner) OutputInDir(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.String(), err
}

func (r *RealProcessRunner) RunInDir(ctx context.Context, dir string, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), err
		}
		return 1, err
	}
	return 0, nil
}
