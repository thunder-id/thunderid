// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withKillPortSeams(t *testing.T) {
	t.Helper()
	originalInUse := killPortInUse
	originalCommand := killPortCommand
	originalSignal := killPortSignal
	originalWait := killPortWait
	t.Cleanup(func() {
		killPortInUse = originalInUse
		killPortCommand = originalCommand
		killPortSignal = originalSignal
		killPortWait = originalWait
	})
}

func TestParseAdminCredentials_ParsesBlock(t *testing.T) {
	output := "some noise\n" +
		"Admin credentials:\n" +
		"  Username: admin\n" +
		"  Password: abc123\n" +
		"  Sign in to the Console with these credentials.\n" +
		"\n" +
		"trailing noise\n"

	creds := parseAdminCredentials(output)

	assert.NotNil(t, creds)
	assert.Equal(t, "admin", creds.Username)
	assert.Equal(t, "abc123", creds.Password)
}

func TestParseAdminCredentials_CRLF(t *testing.T) {
	output := "some noise\r\n" +
		"Admin credentials:\r\n" +
		"  Username: admin\r\n" +
		"  Password: abc123\r\n" +
		"  Sign in to the Console with these credentials.\r\n" +
		"\r\n" +
		"trailing noise\r\n"

	creds := parseAdminCredentials(output)

	assert.NotNil(t, creds)
	assert.Equal(t, "admin", creds.Username)
	assert.Equal(t, "abc123", creds.Password)
}

func TestParseAdminCredentials_NoBlockReturnsNil(t *testing.T) {
	assert.Nil(t, parseAdminCredentials("no credentials here at all"))
}

func TestGenerateAdminPassword(t *testing.T) {
	const special = "@#%+=_.?-"
	for i := 0; i < 100; i++ {
		pw := GenerateAdminPassword()
		assert.Len(t, pw, 12)
		assert.True(t, strings.ContainsAny(pw, "0123456789"), "must contain a digit: %q", pw)
		assert.True(t, strings.ContainsAny(pw, special), "must contain a special char: %q", pw)
	}
	assert.NotEqual(t, GenerateAdminPassword(), GenerateAdminPassword())
}

func TestKillPortWithOS_ReturnsDiscoveryFailure(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	killPortCommand = func(string, ...string) ([]byte, error) {
		return nil, errors.New("lsof unavailable")
	}

	err := killPortWithOS(8090, "linux")

	assert.ErrorContains(t, err, "lsof unavailable")
}

func TestKillPortWithOS_StopsGracefully(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	killPortCommand = func(string, ...string) ([]byte, error) { return []byte("123\n"), nil }
	var signals []syscall.Signal
	killPortSignal = func(pid int, signal syscall.Signal) error {
		assert.Equal(t, 123, pid)
		signals = append(signals, signal)
		return nil
	}
	killPortWait = func(int, time.Duration) bool { return true }

	err := killPortWithOS(8090, "linux")

	assert.NoError(t, err)
	assert.Equal(t, []syscall.Signal{syscall.SIGTERM}, signals)
}

func TestKillPortWithOS_DiscoversOnlyListeningUnixProcesses(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	var gotName string
	var gotArgs []string
	killPortCommand = func(name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = args
		return []byte("123"), nil
	}
	killPortSignal = func(int, syscall.Signal) error { return nil }
	killPortWait = func(int, time.Duration) bool { return true }

	require.NoError(t, killPortWithOS(8090, "linux"))

	assert.Equal(t, "lsof", gotName)
	assert.Equal(t, []string{"-nP", "-tiTCP:8090", "-sTCP:LISTEN"}, gotArgs)
}

func TestKillPortWithOS_EscalatesAfterGracePeriod(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	killPortCommand = func(string, ...string) ([]byte, error) { return []byte("123"), nil }
	var signals []syscall.Signal
	killPortSignal = func(_ int, signal syscall.Signal) error {
		signals = append(signals, signal)
		return nil
	}
	waits := 0
	killPortWait = func(int, time.Duration) bool {
		waits++
		return waits == 2
	}

	err := killPortWithOS(8090, "darwin")

	assert.NoError(t, err)
	assert.Equal(t, []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}, signals)
}

func TestKillPortWithOS_ReturnsErrorWhenListenerSurvives(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	killPortCommand = func(string, ...string) ([]byte, error) { return []byte("123"), nil }
	killPortSignal = func(int, syscall.Signal) error { return nil }
	killPortWait = func(int, time.Duration) bool { return false }

	err := killPortWithOS(8090, "linux")

	assert.ErrorContains(t, err, "port 8090 remains occupied")
}

func TestKillPortWithOS_ReturnsSignalFailure(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	killPortCommand = func(string, ...string) ([]byte, error) { return []byte("123"), nil }
	killPortSignal = func(pid int, signal syscall.Signal) error {
		return fmt.Errorf("signal %d to %d failed", signal, pid)
	}

	err := killPortWithOS(8090, "linux")

	assert.ErrorContains(t, err, "signal")
}

func TestKillPortWithOS_ReturnsWindowsTaskkillFailure(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	killPortCommand = func(name string, _ ...string) ([]byte, error) {
		if name == "powershell.exe" {
			return []byte("123\n"), nil
		}
		return []byte("access denied"), errors.New("exit status 1")
	}

	err := killPortWithOS(8090, "windows")

	assert.ErrorContains(t, err, "access denied")
}

func TestKillPortWithOS_WindowsDiscoversExactListeningPortAndKillsTree(t *testing.T) {
	withKillPortSeams(t)
	killPortInUse = func(int) bool { return true }
	var calls [][]string
	killPortCommand = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if name == "powershell.exe" {
			return []byte("123\n456\n"), nil
		}
		return nil, nil
	}
	killPortWait = func(int, time.Duration) bool { return true }

	require.NoError(t, killPortWithOS(8090, "windows"))

	require.Len(t, calls, 3)
	assert.Equal(t, "powershell.exe", calls[0][0])
	assert.Contains(t, calls[0][len(calls[0])-1], "-LocalPort 8090 -State Listen")
	assert.Equal(t, []string{"taskkill", "/T", "/F", "/PID", "123"}, calls[1])
	assert.Equal(t, []string{"taskkill", "/T", "/F", "/PID", "456"}, calls[2])
}
