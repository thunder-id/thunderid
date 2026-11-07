//go:build !windows

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// sendStopSignal sends SIGTERM to the process for graceful shutdown on Unix.
func sendStopSignal(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

// isProcessAlive reports whether the process identified by proc is still running.
// On Unix the null signal (signal 0) is used: it fails once the process has exited
// and been reaped. A zombie, an exited process its parent has not reaped yet, still
// accepts the null signal, so its process state rules it out.
func isProcessAlive(proc *os.Process) bool {
	if proc.Signal(syscall.Signal(0)) != nil {
		return false
	}
	return !strings.HasPrefix(processStatus(proc.Pid), "Z")
}

// processStatus returns the status of the process, which starts with its state letter, or "" when it
// cannot be read. Linux has it in /proc, and other systems report it through ps.
func processStatus(pid int) string {
	if runtime.GOOS == "linux" {
		stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return ""
		}
		// The state follows the command name, which is in parentheses and can contain them too.
		status := string(stat)
		return strings.TrimSpace(status[strings.LastIndexByte(status, ')')+1:])
	}
	status, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(status))
}
