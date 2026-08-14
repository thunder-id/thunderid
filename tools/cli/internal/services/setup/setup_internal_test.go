// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestParseNetstatPIDs(t *testing.T) {
	out := "  TCP    127.0.0.1:8090         0.0.0.0:0              LISTENING       4321\r\n" +
		// Remote port 8090 on a local port 55000: not a listener on 8090.
		"  TCP    127.0.0.1:55000        10.0.0.5:8090          ESTABLISHED     999\r\n" +
		"garbage line\r\n"

	assert.Equal(t, []int{4321}, parseNetstatPIDs(out, 8090))
}

func TestParsePIDsSkipsInvalid(t *testing.T) {
	assert.Equal(t, []int{7, 9}, parsePIDs([]string{"7", "x", "0", "-3", " 9 "}))
}

func TestFormatPIDs(t *testing.T) {
	assert.Equal(t, "pid 7", formatPIDs([]int{7}))
	assert.Equal(t, "pids 7, 9", formatPIDs([]int{7, 9}))
}

func TestListenerPIDs_FindsOwnListener(t *testing.T) {
	if isWindows() {
		t.Skip("uses lsof")
	}
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck

	pids, err := listenerPIDs(ln.Addr().(*net.TCPAddr).Port)
	require.NoError(t, err)
	assert.Contains(t, pids, os.Getpid())
}

func TestPortHolders_ReportsThisProcess(t *testing.T) {
	if isWindows() {
		t.Skip("uses lsof")
	}
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck
	port := ln.Addr().(*net.TCPAddr).Port

	holders := PortHolders(port)

	require.NotEmpty(t, holders, "expected a holder for the listening port")
	pids := make([]int, 0, len(holders))
	for _, h := range holders {
		assert.Equal(t, port, h.Port)
		pids = append(pids, h.PID)
	}
	assert.Contains(t, pids, os.Getpid())
}

// unboundTestPort is high enough that nothing on a developer machine or CI runner is
// expected to listen on it, and it is never bound by these tests. Asserting on a port
// that was just closed would instead race whatever binds it next.
const unboundTestPort = 19997

func TestPortHolders_SkipsFreePorts(t *testing.T) {
	assert.Empty(t, PortHolders(unboundTestPort))
}

func TestPortHolderString(t *testing.T) {
	assert.Equal(t, "5173  node (pid 61201)", PortHolder{Port: 5173, PID: 61201, Name: "node"}.String())
	assert.Equal(t, "5173  pid 61201", PortHolder{Port: 5173, PID: 61201}.String())
	assert.Equal(t, "5173  unknown process", PortHolder{Port: 5173}.String())
}

func TestParseTasklistName(t *testing.T) {
	assert.Equal(t, "node.exe", parseTasklistName("\"node.exe\",\"61201\",\"Console\",\"1\",\"52,300 K\"\r\n"))
	// tasklist exits 0 and prints this when nothing matched the filter.
	assert.Empty(t, parseTasklistName("INFO: No tasks are running which match the specified criteria.\r\n"))
	assert.Empty(t, parseTasklistName(""))
}

func TestProcessName_ReturnsOwnBinary(t *testing.T) {
	if isWindows() {
		t.Skip("uses ps")
	}
	name := processName(os.Getpid())
	assert.NotEmpty(t, name)
	assert.NotContains(t, name, " ")
	assert.NotContains(t, name, "/")
}

func TestFreePort_NoListenerIsNoOp(t *testing.T) {
	require.NoError(t, FreePort(unboundTestPort, 2*time.Second))
	require.NoError(t, KillPort(unboundTestPort))
}

// An empty result is "nothing is listening", which callers must not confuse with the
// lookup itself failing: that difference decides whether a port reports an unknown
// holder or none at all.
func TestListenerPIDs_NoListener(t *testing.T) {
	pids, err := listenerPIDs(unboundTestPort)
	require.NoError(t, err, "nothing listening is not a lookup failure")
	assert.Empty(t, pids)
}
