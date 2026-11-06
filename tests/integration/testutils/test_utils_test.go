// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// standInServerEnv makes the test binary run TestStandInServerProcess as a stand-in for the server.
const standInServerEnv = "TESTUTILS_STAND_IN_SERVER"

// standInServerDrain is how long the stand-in server keeps running after it stops listening.
const standInServerDrain = time.Second

// standInServerStopBound is how long stopping the stand-in server may take: its drain, with room for a
// busy machine, and well short of waiting out serverStopGracePeriod on any process.
const standInServerStopBound = standInServerDrain + 2*time.Second

type TestUtilsTestSuite struct {
	suite.Suite
}

func TestTestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(TestUtilsTestSuite))
}

func (ts *TestUtilsTestSuite) TestWithoutTopLevelKeyDropsTheWholeBlock() {
	doc := "id: user-1\nsharingPolicies:\n  - childUnits:\n      mode: all\n\n  - otherTreeRoots:\n      mode: all\nattributes:\n  username: alice\n"

	got := WithoutTopLevelKey(doc, "sharingPolicies")

	ts.Equal("id: user-1\nattributes:\n  username: alice\n", got)
}

func (ts *TestUtilsTestSuite) TestWithoutTopLevelKeyLeavesOtherKeysAlone() {
	doc := "id: user-1\nattributes:\n  id: nested\n"

	ts.Equal("attributes:\n  id: nested\n", WithoutTopLevelKey(doc, "id"))
}

func (ts *TestUtilsTestSuite) TestServerPortIgnoresAServerPortTheRunnerDidNotSet() {
	ts.T().Setenv(TestGroupEnv, "")
	ts.T().Setenv("SERVER_EXTRACTED_HOME", "")
	ts.T().Setenv("SERVER_PORT", "3000")

	ts.Equal(DefaultServerPort, ServerPort())
}

func (ts *TestUtilsTestSuite) TestServerPortUsesTheTestGroupPort() {
	ts.T().Setenv(TestGroupEnv, "g1")
	ts.T().Setenv("SERVER_EXTRACTED_HOME", "")
	ts.T().Setenv("SERVER_PORT", "8195")

	ts.Equal("8195", ServerPort())
}

func (ts *TestUtilsTestSuite) TestServerPortUsesThePortTheRunnerPassesToTestPackages() {
	ts.T().Setenv(TestGroupEnv, "")
	ts.T().Setenv("SERVER_EXTRACTED_HOME", "/tmp/product")
	ts.T().Setenv("SERVER_PORT", "8195")

	ts.Equal("8195", ServerPort())
}

// A test package's first stop knows the server from main.go's SERVER_PID, which names main.go's first
// server long after another package replaced it: that process has exited, and main.go has not reaped it.
// The running server is known only through the PID file.
func (ts *TestUtilsTestSuite) TestStopServerWaitsForAServerKnownOnlyByThePIDFile() {
	ts.skipWithoutUnixSignals()
	replaced := exec.Command("true")
	replacedOutput, err := replaced.StdoutPipe()
	ts.Require().NoError(err)
	ts.Require().NoError(replaced.Start())
	ts.T().Cleanup(func() { _ = replaced.Wait() })
	// Its output closes when it exits, which leaves it unreaped until the cleanup above.
	_, err = io.ReadAll(replacedOutput)
	ts.Require().NoError(err)

	server, conn, port := ts.startStandInServer()
	ts.useScratchServerState(port)
	serverPid = replaced.Process.Pid
	writePidFile(server.Process.Pid)

	started := time.Now()
	StopServer()

	ts.Less(time.Since(started), standInServerStopBound, "StopServer waited on a process that had exited")
	ts.assertStandInExited(conn)
	ts.NoFileExists(pidFilePath())
}

func (ts *TestUtilsTestSuite) TestStopServerWaitsForAServerKnownByItsPID() {
	ts.skipWithoutUnixSignals()
	server, conn, port := ts.startStandInServer()
	ts.useScratchServerState(port)
	serverPid = server.Process.Pid

	started := time.Now()
	StopServer()

	ts.Less(time.Since(started), standInServerStopBound, "StopServer waited on a process that had exited")
	ts.assertStandInExited(conn)
}

func (ts *TestUtilsTestSuite) TestIsProcessAliveUntilTheProcessExits() {
	server, _, _ := ts.startStandInServer()
	ts.True(isProcessAlive(server.Process))

	ts.Require().NoError(server.Process.Kill())

	ts.True(waitForProcessExit(server.Process, 5*time.Second), "a killed child this process has not reaped")
}

// A server that has closed its listener but still holds a connection passes waitForPortClosed and
// fails the setup script's port check.
func (ts *TestUtilsTestSuite) TestWaitForSetupPortFreeNamesWhatHoldsThePort() {
	if _, err := exec.LookPath("lsof"); err != nil || runtime.GOOS == "windows" {
		ts.T().Skip("the setup script checks socket holders with lsof")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	ts.Require().NoError(err)
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	client, err := net.Dial("tcp", ln.Addr().String())
	ts.Require().NoError(err)
	accepted, err := ln.Accept()
	ts.Require().NoError(err)
	ts.Require().NoError(ln.Close())
	ts.useScratchServerState(port)
	ts.Require().NoError(os.WriteFile(filepath.Join(extractedProductHome, "deployment.yaml"),
		[]byte("server:\n  port: "+port+"\n"), 0o600))
	ts.Require().NoError(waitForPortClosed(port, time.Second))

	err = waitForSetupPortFree(500 * time.Millisecond)

	ts.ErrorContains(err, "port "+port+" was still in use")
	ts.ErrorContains(err, strconv.Itoa(os.Getpid()))

	ts.Require().NoError(accepted.Close())
	ts.Require().NoError(client.Close())
	ts.NoError(waitForSetupPortFree(5 * time.Second))
}

// TestStandInServerProcess is the server the StopServer tests stop, not a test. Like the real server,
// it closes its listener as soon as it gets SIGTERM, and exits only after a drain, holding the
// connection it accepted open until then.
func (ts *TestUtilsTestSuite) TestStandInServerProcess() {
	if os.Getenv(standInServerEnv) == "" {
		ts.T().Skip("runs only as the stand-in server of the StopServer tests")
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(ln.Addr().(*net.TCPAddr).Port)
	conn, err := ln.Accept()
	if err != nil {
		os.Exit(1)
	}

	<-stop
	_ = ln.Close()
	time.Sleep(standInServerDrain)
	_ = conn.Close()
	os.Exit(0)
}

func (ts *TestUtilsTestSuite) skipWithoutUnixSignals() {
	if runtime.GOOS == "windows" {
		ts.T().Skip("the stand-in server shuts down on SIGTERM, which Windows cannot send")
	}
}

// startStandInServer starts the stand-in server as a child of this process, which never reaps it while
// StopServer runs, and returns it with the port it listens on and a connection to it.
func (ts *TestUtilsTestSuite) startStandInServer() (*exec.Cmd, net.Conn, string) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestTestUtilsTestSuite$", "-testify.m=^TestStandInServerProcess$",
		"-test.timeout=1m")
	cmd.Env = append(os.Environ(), standInServerEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	ts.Require().NoError(err)
	ts.Require().NoError(cmd.Start())
	ts.T().Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	port, err := bufio.NewReader(stdout).ReadString('\n')
	ts.Require().NoError(err)
	port = strings.TrimSpace(port)
	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = conn.Close() })
	return cmd, conn, port
}

// useScratchServerState points the server state StopServer reads at port and an empty product home,
// so the server under test is never touched, and restores the state when the test ends.
func (ts *TestUtilsTestSuite) useScratchServerState(port string) {
	savedPort, savedHome, savedCmd, savedPid, savedInit := serverPort, extractedProductHome, serverCmd, serverPid,
		isInitialized
	ts.T().Cleanup(func() {
		serverPort, extractedProductHome, serverCmd, serverPid, isInitialized = savedPort, savedHome, savedCmd,
			savedPid, savedInit
	})
	serverPort, extractedProductHome, serverCmd, serverPid, isInitialized = port, ts.T().TempDir(), nil, 0, true
}

// assertStandInExited checks that the stand-in server's connection has been closed, which happens
// when the stand-in exits and not before.
func (ts *TestUtilsTestSuite) assertStandInExited(conn net.Conn) {
	ts.Require().NoError(conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)))
	_, err := conn.Read(make([]byte, 1))
	ts.ErrorIs(err, io.EOF, "StopServer returned while the server was still running")
}
