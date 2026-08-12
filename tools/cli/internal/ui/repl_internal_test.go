// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/commands/sample"
	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close() //nolint:errcheck
	os.Stdout = orig
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	return string(out)
}

func TestPrintCredentialsFallback_PrintsValues(t *testing.T) {
	creds := &setup.AdminCredentials{Username: "admin", Password: "s3cr3t#Pass"}
	out := captureStdout(t, func() { PrintCredentialsFallback(creds) })
	if !strings.Contains(out, "admin") || !strings.Contains(out, "s3cr3t#Pass") {
		t.Fatalf("fallback output missing credentials:\n%s", out)
	}
}

func TestPrintCredentialsFallback_NilIsNoOp(t *testing.T) {
	out := captureStdout(t, func() { PrintCredentialsFallback(nil) })
	if out != "" {
		t.Fatalf("expected no output for nil creds, got:\n%s", out)
	}
}

func TestCredentialsBox_RendersValues(t *testing.T) {
	creds := &setup.AdminCredentials{Username: "admin", Password: "s3cr3t#Pass"}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, creds)
	box := m.credentialsBox()
	if !strings.Contains(box, "admin") || !strings.Contains(box, "s3cr3t#Pass") {
		t.Fatalf("credentials box missing values:\n%s", box)
	}
}

func TestCredentialsBox_NilReturnsEmpty(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	if box := m.credentialsBox(); box != "" {
		t.Fatalf("expected empty box for nil creds, got:\n%s", box)
	}
}

func TestRender_IncludesCredentials(t *testing.T) {
	creds := &setup.AdminCredentials{Username: "admin", Password: "s3cr3t#Pass"}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, creds)
	out := m.render()
	if !strings.Contains(out, "admin") || !strings.Contains(out, "s3cr3t#Pass") {
		t.Fatalf("render output missing credentials:\n%s", out)
	}
}

// updateModel applies msg and returns the resulting model.
func updateModel(t *testing.T, m ReplModel, msg tea.Msg) ReplModel {
	t.Helper()
	updated, _ := m.Update(msg)
	next, ok := updated.(ReplModel)
	if !ok {
		t.Fatalf("Update returned %T, want ReplModel", updated)
	}
	return next
}

// Bracketed paste is delivered as its own message, not as key presses. An API key
// is too long to retype, so the config prompt has to accept a pasted value.
func TestPaste_ReachesUsecaseConfigInput(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.showUsecaseConfig = true
	m.ucInputs = []ConfigInput{{Key: "LLM_API_KEY", Label: "API key", Secret: true}}
	m.ucStep = 0
	m.initUCStep()

	next := updateModel(t, m, tea.PasteMsg{Content: "pasted-api-key"})
	if got := next.ucText.Value(); got != "pasted-api-key" {
		t.Fatalf("pasted value did not reach the config input: got %q", got)
	}
}

func TestPaste_ReachesCommandInput(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.input.Focus()

	next := updateModel(t, m, tea.PasteMsg{Content: "/try-agentid"})
	if got := next.input.Value(); got != "/try-agentid" {
		t.Fatalf("pasted value did not reach the command input: got %q", got)
	}
}

// A list step has no text input to paste into, so the paste must be ignored
// rather than routed at the wrong widget.
func TestPaste_IgnoredOnChoiceStep(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.showUsecaseConfig = true
	m.ucInputs = []ConfigInput{{
		Key:     "LLM_PROVIDER",
		Label:   "Provider",
		Choices: []Choice{{Value: "anthropic", Label: "Anthropic"}},
	}}
	m.ucStep = 0
	m.initUCStep()

	next := updateModel(t, m, tea.PasteMsg{Content: "anthropic"})
	if got := next.ucText.Value(); got != "" {
		t.Fatalf("paste must not reach the text input on a choice step: got %q", got)
	}
	if got := next.input.Value(); got != "" {
		t.Fatalf("paste must not leak to the command input: got %q", got)
	}
}

func TestStop_AttachedSessionUsesConfiguredServerStopper(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	stopped := false
	m.stopServer = func() error {
		stopped = true
		return nil
	}

	cmd := m.runCommand("/stop")

	if !stopped || !m.quitting || cmd == nil {
		t.Fatalf("stopped=%v quitting=%v cmd=%v", stopped, m.quitting, cmd)
	}
}

func TestStop_ShowsTerminationErrorAndKeepsSessionOpen(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.stopServer = func() error { return errors.New("listener survived") }

	cmd := m.runCommand("/stop")

	if cmd != nil || m.quitting {
		t.Fatalf("failed stop must keep REPL open: quitting=%v cmd=%v", m.quitting, cmd)
	}
	if got := strings.Join(m.messages, "\n"); !strings.Contains(got, "listener survived") {
		t.Fatalf("termination error not shown: %s", got)
	}
}

func TestStop_CleansSampleBeforeServer(t *testing.T) {
	originalStopSample := stopSampleProcess
	t.Cleanup(func() { stopSampleProcess = originalStopSample })
	var order []string
	stopSampleProcess = func(*sample.Process) error {
		order = append(order, "sample")
		return nil
	}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.sampleProc = &sample.Process{}
	m.stopServer = func() error {
		order = append(order, "server")
		return nil
	}

	m.runCommand("/stop")

	if got := strings.Join(order, ","); got != "sample,server" {
		t.Fatalf("shutdown order: got %q", got)
	}
}

func TestUpgrade_CleansSampleBeforeTransition(t *testing.T) {
	originalStopSample := stopSampleProcess
	t.Cleanup(func() { stopSampleProcess = originalStopSample })
	cleaned := false
	stopSampleProcess = func(*sample.Process) error {
		cleaned = true
		return nil
	}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.sampleProc = &sample.Process{}

	next := updateModel(t, m, upgradeMsg{})

	if !cleaned || !next.upgradeRequested || !next.quitting {
		t.Fatalf("cleaned=%v upgrade=%v quitting=%v", cleaned, next.upgradeRequested, next.quitting)
	}
}

func TestSwitch_CleansSampleBeforeTransition(t *testing.T) {
	originalStopSample := stopSampleProcess
	t.Cleanup(func() { stopSampleProcess = originalStopSample })
	cleaned := false
	stopSampleProcess = func(*sample.Process) error {
		cleaned = true
		return nil
	}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.sampleProc = &sample.Process{}

	next := updateModel(t, m, switchVersionMsg{})

	if !cleaned || !next.switchRequested || !next.quitting {
		t.Fatalf("cleaned=%v switch=%v quitting=%v", cleaned, next.switchRequested, next.quitting)
	}
}

func TestCtrlC_CleansSampleAndStopsServer(t *testing.T) {
	originalStopSample := stopSampleProcess
	t.Cleanup(func() { stopSampleProcess = originalStopSample })
	var order []string
	stopSampleProcess = func(*sample.Process) error {
		order = append(order, "sample")
		return nil
	}
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.sampleProc = &sample.Process{}
	m.stopServer = func() error {
		order = append(order, "server")
		return nil
	}

	next := updateModel(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if got := strings.Join(order, ","); got != "sample,server" {
		t.Fatalf("shutdown order: got %q", got)
	}
	if !next.quitting {
		t.Fatal("Ctrl+C did not quit after cleanup")
	}
}

func TestCtrlC_ExitsWhenServerAlreadyStopped(t *testing.T) {
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.status = statusStopped
	called := false
	m.stopServer = func() error {
		called = true
		return nil
	}

	next := updateModel(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if !next.quitting {
		t.Fatal("Ctrl+C did not exit stopped session")
	}
	if !called {
		t.Fatal("stopped status skipped server verification")
	}
}

func TestSampleDone_ReplacesOwnedLauncherStopper(t *testing.T) {
	originalStopServerProcess := stopServerProcess
	t.Cleanup(func() { stopServerProcess = originalStopServerProcess })
	oldCmd := &exec.Cmd{Process: &os.Process{Pid: 101}}
	newCmd := &exec.Cmd{Process: &os.Process{Pid: 202}}
	var stopped *exec.Cmd
	stopServerProcess = func(cmd *exec.Cmd) error {
		stopped = cmd
		return nil
	}
	m := NewReplModel("1.0.0", oldCmd, "/tmp/x", false, false, nil)

	m = updateModel(t, m, sampleDoneMsg{proc: newCmd, serverURL: "http://localhost:8090"})
	if err := m.stopServer(); err != nil {
		t.Fatalf("stop replacement launcher: %v", err)
	}

	if stopped != newCmd {
		t.Fatalf("stopper retained stale launcher: got %p want %p", stopped, newCmd)
	}
}

func TestSampleError_ReplacesOwnedLauncherStopper(t *testing.T) {
	originalStopServerProcess := stopServerProcess
	t.Cleanup(func() { stopServerProcess = originalStopServerProcess })
	oldCmd := &exec.Cmd{Process: &os.Process{Pid: 101}}
	newCmd := &exec.Cmd{Process: &os.Process{Pid: 202}}
	var stopped *exec.Cmd
	stopServerProcess = func(cmd *exec.Cmd) error {
		stopped = cmd
		return nil
	}
	m := NewReplModel("1.0.0", oldCmd, "/tmp/x", false, false, nil)

	m = updateModel(t, m, sampleErrMsg{err: errors.New("readiness failed"), proc: newCmd})
	if err := m.stopServer(); err != nil {
		t.Fatalf("stop replacement launcher: %v", err)
	}

	if stopped != newCmd {
		t.Fatalf("stopper retained stale launcher: got %p want %p", stopped, newCmd)
	}
}
