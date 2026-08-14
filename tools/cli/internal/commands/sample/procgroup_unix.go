// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sample

import (
	"os/exec"
	"syscall"
	"time"
)

// killGracePeriod is how long the process group gets to exit after SIGTERM
// before it is killed outright.
const killGracePeriod = 3 * time.Second

// setProcessGroup puts cmd in its own process group so every descendant npm
// spawns can be signaled as a unit.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup terminates cmd's process group, escalating to SIGKILL if the
// group is still alive after killGracePeriod.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// setProcessGroup made the process its own group leader, so the group id is
	// its pid. Do not look it up with Getpgid: once npm itself has exited and
	// been reaped, that returns ESRCH even though the group still has members.
	pgid := cmd.Process.Pid
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return
	}
	deadline := time.Now().Add(killGracePeriod)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-pgid, 0); err != nil {
			return // group is gone
		}
		time.Sleep(100 * time.Millisecond)
	}
	syscall.Kill(-pgid, syscall.SIGKILL) //nolint:errcheck
}
