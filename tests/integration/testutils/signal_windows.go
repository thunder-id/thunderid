//go:build windows

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"os"
	"syscall"
)

// sendStopSignal kills the process on Windows.
// Windows does not support sending signals (SIGTERM/SIGINT) to other processes
// via Process.Signal(). The only reliable approach is Process.Kill().
func sendStopSignal(process *os.Process) error {
	return process.Kill()
}

// isProcessAlive reports whether the process identified by proc is still running.
// os.FindProcess opens an HANDLE via OpenProcess; the kernel will not recycle the
// PID while any handle to the process object remains open, so the PID still names
// the original process, whose handle is signalled once it has exited.
func isProcessAlive(proc *os.Process) bool {
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(proc.Pid))
	if err != nil {
		return false
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	event, err := syscall.WaitForSingleObject(handle, 0)
	return err == nil && event == syscall.WAIT_TIMEOUT
}
