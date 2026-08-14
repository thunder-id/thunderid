// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sample

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// resetRunning clears the tracked-process state around a test.
func resetRunning(t *testing.T) {
	t.Helper()
	clear := func() {
		running.Lock()
		defer running.Unlock()
		running.cmd, running.stopping = nil, false
	}
	clear()
	t.Cleanup(clear)
}

// alive reports whether pid is a running process. A terminated process stays
// visible to Kill(pid, 0) until its parent reaps it, so the process state has to
// be checked too: otherwise a not-yet-reaped zombie reads as alive and the test
// races the reaper.
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false // ps exits non-zero when the pid is gone
	}
	state := strings.TrimSpace(string(out))
	return state != "" && !strings.HasPrefix(state, "Z")
}

// startTree starts a shell in its own process group that spawns a grandchild
// and writes both PIDs to files, mirroring how npm spawns the sample's services.
func startTree(t *testing.T) (cmd *exec.Cmd, childPID, grandchildPID int) {
	t.Helper()
	dir := t.TempDir()
	grandPIDFile := filepath.Join(dir, "grandchild.pid")
	script := "sh -c 'echo $$ > " + grandPIDFile + "; sleep 300' & sleep 300"

	cmd = exec.Command("sh", "-c", script)
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { killProcessGroup(cmd) })

	return cmd, cmd.Process.Pid, readPID(t, grandPIDFile)
}

// readPID waits for path to hold a pid and returns it.
func readPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(string(data))); convErr == nil && pid > 0 {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no pid reported in %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitGone polls until pid is gone or the deadline passes.
func waitGone(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !alive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !alive(pid)
}

func TestStopServicesKillsProcessTree(t *testing.T) {
	resetRunning(t)

	cmd, childPID, grandchildPID := startTree(t)
	go cmd.Wait() //nolint:errcheck // reap the child so it does not linger as a zombie
	if !trackRunning(cmd) {
		t.Fatal("trackRunning refused a process outside shutdown")
	}

	if !alive(grandchildPID) {
		t.Fatalf("grandchild %d not running before StopServices", grandchildPID)
	}

	StopServices()

	if !waitGone(childPID, 10*time.Second) {
		t.Errorf("child %d still alive after StopServices", childPID)
	}
	if !waitGone(grandchildPID, 10*time.Second) {
		t.Errorf("grandchild %d still alive after StopServices", grandchildPID)
	}

	// A second call must be a safe no-op: nothing is tracked any more.
	done := make(chan struct{})
	go func() { StopServices(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("second StopServices call blocked")
	}
}

func TestStopServicesWithoutStart(t *testing.T) {
	resetRunning(t)

	done := make(chan struct{})
	go func() { StopServices(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StopServices blocked with nothing started")
	}
}

// Stopping the sample must not touch a listener on one of the sample's ports
// that this CLI did not start. A regressed port sweep would signal the process
// holding it, which here is the test binary itself.
func TestStopServicesLeavesForeignListenerAlone(t *testing.T) {
	resetRunning(t)

	var ln net.Listener
	for _, port := range sampleServicePorts(true) {
		l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			continue // in use by something else on this machine
		}
		ln = l
		break
	}
	if ln == nil {
		t.Skip("no sample port free to bind")
	}
	defer ln.Close() //nolint:errcheck

	cmd, childPID, _ := startTree(t)
	go cmd.Wait() //nolint:errcheck
	if !trackRunning(cmd) {
		t.Fatal("trackRunning refused a process outside shutdown")
	}

	StopServices()

	if !waitGone(childPID, 10*time.Second) {
		t.Errorf("child %d still alive after StopServices", childPID)
	}
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("listener on %s was stopped: %v", ln.Addr(), err)
	}
	conn.Close() //nolint:errcheck
}

// TestKillProcessGroupAfterLeaderExit covers the case where npm itself has
// exited and been reaped while its children are still running: the group id
// must come from the recorded pid, not from Getpgid (which returns ESRCH).
func TestKillProcessGroupAfterLeaderExit(t *testing.T) {
	dir := t.TempDir()
	grandPIDFile := filepath.Join(dir, "grandchild.pid")

	// The leader spawns a child and exits immediately.
	cmd := exec.Command("sh", "-c", "sh -c 'echo $$ > "+grandPIDFile+"; sleep 300' & exit 0")
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	leaderPID := cmd.Process.Pid
	t.Cleanup(func() { killProcessGroup(cmd) })

	grandchildPID := readPID(t, grandPIDFile)
	if err := cmd.Wait(); err != nil { // reap the leader, so Getpgid(leaderPID) now fails
		t.Fatalf("wait: %v", err)
	}
	if !waitGone(leaderPID, 5*time.Second) {
		t.Fatalf("leader %d still alive after Wait", leaderPID)
	}
	if !alive(grandchildPID) {
		t.Fatalf("grandchild %d already gone, nothing to test", grandchildPID)
	}

	killProcessGroup(cmd)

	if !waitGone(grandchildPID, 10*time.Second) {
		t.Errorf("grandchild %d survived killProcessGroup after the leader exited", grandchildPID)
	}
}

// TestTrackRunningRefusedDuringShutdown covers a sample that finishes starting
// after StopServices already ran: it must not be recorded, and the caller
// terminates it.
func TestTrackRunningRefusedDuringShutdown(t *testing.T) {
	resetRunning(t)

	StopServices() // latches shutdown

	cmd, childPID, grandchildPID := startTree(t)
	go cmd.Wait() //nolint:errcheck
	if trackRunning(cmd) {
		t.Fatal("trackRunning accepted a process after shutdown began")
	}
	killProcessGroup(cmd) // what startSampleServices does on refusal

	if !waitGone(childPID, 10*time.Second) {
		t.Errorf("child %d still alive", childPID)
	}
	if !waitGone(grandchildPID, 10*time.Second) {
		t.Errorf("grandchild %d still alive", grandchildPID)
	}

	running.Lock()
	got := running.cmd
	running.Unlock()
	if got != nil {
		t.Errorf("a refused process was recorded: %v", got)
	}
}

func TestBeginRunClearsShutdownLatch(t *testing.T) {
	resetRunning(t)

	StopServices()
	beginRun()

	cmd := exec.Command("true")
	if !trackRunning(cmd) {
		t.Fatal("trackRunning still refuses after beginRun")
	}
}

// A second sample run must stop the first one: trackRunning replaces the tracked
// handle, so an unstopped first process group could never be signaled again.
func TestBeginRunStopsThePreviousRun(t *testing.T) {
	resetRunning(t)

	cmd, childPID, grandchildPID := startTree(t)
	go cmd.Wait() //nolint:errcheck
	if !trackRunning(cmd) {
		t.Fatal("trackRunning refused a process outside shutdown")
	}

	beginRun()

	if !waitGone(childPID, 10*time.Second) {
		t.Errorf("previous child %d still alive after beginRun", childPID)
	}
	if !waitGone(grandchildPID, 10*time.Second) {
		t.Errorf("previous grandchild %d still alive after beginRun", grandchildPID)
	}
	if !trackRunning(exec.Command("true")) {
		t.Fatal("beginRun left the shutdown latch set")
	}
}

func TestClearRunningOnlyClearsMatchingCmd(t *testing.T) {
	resetRunning(t)

	current := exec.Command("true")
	trackRunning(current)
	clearRunning(exec.Command("true")) // a different, older command

	running.Lock()
	got := running.cmd
	running.Unlock()
	if got != current {
		t.Errorf("clearRunning dropped a handle it did not own")
	}
}
