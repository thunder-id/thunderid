// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package sample

import (
	"fmt"
	"os/exec"
)

// setProcessGroup is a no-op on Windows; process trees are terminated with
// taskkill /T instead of a process-group signal.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup terminates cmd and its whole process tree.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
}
