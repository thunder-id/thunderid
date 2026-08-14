// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// keyPress builds the key message the REPL receives for a named key.
func keyPress(name string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: keyCodeFor(name), Mod: keyModFor(name)}
}

func keyCodeFor(name string) rune {
	switch name {
	case "pgup":
		return tea.KeyPgUp
	case "pgdown":
		return tea.KeyPgDown
	case "shift+up", "up":
		return tea.KeyUp
	case "shift+down", "down":
		return tea.KeyDown
	case "enter":
		return tea.KeyEnter
	case "esc":
		return tea.KeyEscape
	}
	return 0
}

func keyModFor(name string) tea.KeyMod {
	if strings.HasPrefix(name, "shift+") {
		return tea.ModShift
	}
	return 0
}

// longOutputModel returns a ready REPL sized to a short terminal, holding more output
// than fits on screen.
func longOutputModel(t *testing.T, lines int) ReplModel {
	t.Helper()
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.status = statusReady
	m.input.Focus()
	for i := 0; i < lines; i++ {
		m.messages = append(m.messages, fmt.Sprintf("output line %d", i))
	}
	return updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
}

// Output longer than the terminal used to push the input line off screen; the view
// must stay within the window and keep the prompt visible.
func TestRender_LongOutputKeepsInputOnScreen(t *testing.T) {
	m := longOutputModel(t, 200)
	out := m.render()

	if height := lipgloss.Height(out); height > 24 {
		t.Fatalf("view is %d lines tall, does not fit a 24-line terminal:\n%s", height, out)
	}
	if !strings.Contains(out, m.input.Prompt) {
		t.Fatalf("input prompt is not visible:\n%s", out)
	}
	if !strings.Contains(out, "output line 199") {
		t.Fatalf("expected the newest output to be visible:\n%s", out)
	}
}

// Earlier output stays reachable: PgUp scrolls back, PgDn returns to the newest lines.
func TestScrollKeys_RevealEarlierOutputAndReturn(t *testing.T) {
	m := longOutputModel(t, 200)

	scrolledUp := m
	for i := 0; i < 10; i++ {
		scrolledUp = updateModel(t, scrolledUp, keyPress("pgup"))
	}
	up := scrolledUp.render()
	if strings.Contains(up, "output line 199") {
		t.Fatalf("PgUp did not move away from the newest output:\n%s", up)
	}
	if !strings.Contains(up, m.input.Prompt) {
		t.Fatalf("input prompt must stay pinned while scrolling:\n%s", up)
	}

	back := scrolledUp
	for i := 0; i < 20; i++ {
		back = updateModel(t, back, keyPress("pgdown"))
	}
	if out := back.render(); !strings.Contains(out, "output line 199") {
		t.Fatalf("PgDn did not return to the newest output:\n%s", out)
	}
}

// Scroll keys must not leak into the focused command input.
func TestScrollKeys_DoNotReachTheInput(t *testing.T) {
	m := longOutputModel(t, 50)
	m.input.SetValue("/try")

	next := updateModel(t, m, keyPress("pgup"))
	if got := next.input.Value(); got != "/try" {
		t.Fatalf("scroll key changed the input value: got %q", got)
	}
}

// New output while the user reads earlier output must not yank the view to the bottom.
func TestNewOutput_DoesNotStealScrollPosition(t *testing.T) {
	m := longOutputModel(t, 200)
	for i := 0; i < 10; i++ {
		m = updateModel(t, m, keyPress("pgup"))
	}
	before := m.render()

	m.messages = append(m.messages, "fresh output")
	m = updateModel(t, m, sampleProgressMsg{line: "working"})

	if after := m.render(); after != before {
		t.Fatalf("scroll position moved on new output:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// stubPortHolders replaces the port lookup for the duration of a test, so the overlay
// does not depend on what is listening on the machine running the tests.
func stubPortHolders(t *testing.T, holders []setup.PortHolder) {
	t.Helper()
	orig := portHolders
	portHolders = func(_ ...int) []setup.PortHolder { return holders }
	t.Cleanup(func() { portHolders = orig })
}

// A sample run frees the dev ports it needs, so an unrelated process holding one of
// them must not be stopped before the user has been asked.
func TestLaunchTry_HeldPortsOpenTheOverlayInsteadOfStarting(t *testing.T) {
	stubPortHolders(t, []setup.PortHolder{{Port: 5173, PID: 4242, Name: "node"}})
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)

	if cmd := m.launchTry("wayfinder", sample.Options{}); cmd != nil {
		t.Fatal("expected no launch command while the conflict is unresolved")
	}
	if !m.showPortConflict {
		t.Fatal("expected the port-conflict overlay to open")
	}
	if m.tryingOut {
		t.Fatal("the sample must not start before the user approves")
	}
	if view := renderPortConflict(m); !strings.Contains(view, "5173") || !strings.Contains(view, "4242") {
		t.Fatalf("overlay does not name the port holder:\n%s", view)
	}
}

func TestLaunchTry_FreePortsStartImmediately(t *testing.T) {
	stubPortHolders(t, nil)
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)

	if cmd := m.launchTry("wayfinder", sample.Options{}); cmd == nil {
		t.Fatal("expected the sample to start when no port is held")
	}
	if m.showPortConflict {
		t.Fatal("there is no conflict to show")
	}
	if !m.tryingOut {
		t.Fatal("expected tryingOut to be set")
	}
}

func TestPortConflictOverlay_ApproveStartsTheSample(t *testing.T) {
	stubPortHolders(t, []setup.PortHolder{{Port: 8787, PID: 51, Name: "node"}})
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.launchTry("wayfinder", sample.Options{})

	next := updateModel(t, m, keyPress("enter"))

	if next.showPortConflict {
		t.Fatal("expected the overlay to close on Enter")
	}
	if !next.tryingOut {
		t.Fatal("expected the sample to start after approval")
	}
}

func TestPortConflictOverlay_CancelLeavesHoldersRunning(t *testing.T) {
	stubPortHolders(t, []setup.PortHolder{{Port: 8787, PID: 51, Name: "node"}})
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.launchTry("wayfinder", sample.Options{})

	// Move the selection off the default answer, then confirm.
	m = updateModel(t, m, keyPress("down"))
	if m.pcStop {
		t.Fatal("expected the selection to move to Cancel")
	}
	next := updateModel(t, m, keyPress("enter"))

	if next.showPortConflict {
		t.Fatal("expected the overlay to close on Enter")
	}
	if next.tryingOut {
		t.Fatal("the sample must not start after canceling")
	}
	if len(next.messages) == 0 || !strings.Contains(next.messages[len(next.messages)-1], "8787") {
		t.Fatalf("expected a message naming the untouched port, got %v", next.messages)
	}
}

// runCommand latches tryingOut before the try is dispatched. Cancelling the overlay
// returns to the prompt, so a latch left set would reject every later command.
func TestPortConflictOverlay_CancelReleasesTheCommandLatch(t *testing.T) {
	stubPortHolders(t, []setup.PortHolder{{Port: 5173, PID: 61, Name: "node"}})
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.tryingOut = true // as runCommand sets it before dispatching the try
	m.launchTry("wayfinder", sample.Options{})

	next := updateModel(t, m, keyPress("esc"))

	if next.tryingOut {
		t.Fatal("the latch must be released when the overlay opens")
	}
	next.runCommand("/nope")
	last := next.messages[len(next.messages)-1]
	if strings.Contains(last, "setup is in progress") {
		t.Fatalf("commands are still rejected after cancelling: %q", last)
	}
}

func TestPortConflictOverlay_EscDismisses(t *testing.T) {
	stubPortHolders(t, []setup.PortHolder{{Port: 2525, PID: 52}})
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.launchTry("wayfinder", sample.Options{})

	next := updateModel(t, m, keyPress("esc"))

	if next.showPortConflict || next.tryingOut {
		t.Fatalf("esc must dismiss the overlay without starting: conflict=%v trying=%v",
			next.showPortConflict, next.tryingOut)
	}
}

// The use-case config flow launches once its values are collected, and must go
// through the same approval as a direct /try.
func TestAdvanceUCStep_LastStepChecksPorts(t *testing.T) {
	stubPortHolders(t, []setup.PortHolder{{Port: 5173, PID: 53, Name: "node"}})
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.showUsecaseConfig = true
	m.ucInputs = []ConfigInput{{Key: "LLM_API_KEY", Label: "API key"}}
	m.ucValues = map[string]string{}
	m.ucLaunch = func(values map[string]string) (string, sample.Options) {
		return "wayfinder", sample.Options{Config: values, Features: []string{"ai"}}
	}

	if cmd := m.advanceUCStep("key-123"); cmd != nil {
		t.Fatal("expected no launch command while the conflict is unresolved")
	}
	if !m.showPortConflict {
		t.Fatal("expected the port-conflict overlay after the last config step")
	}
	if m.showUsecaseConfig {
		t.Fatal("expected the config overlay to close")
	}
}

// stubStopPort replaces the port-stop hooks and returns the ports it was asked to stop.
func stubStopPort(t *testing.T, ready bool) *[]int {
	t.Helper()
	origReady, origStop := productOnPort, stopPort
	stopped := []int{}
	productOnPort = func(int) bool { return ready }
	stopPort = func(port int) error {
		stopped = append(stopped, port)
		return nil
	}
	t.Cleanup(func() { productOnPort, stopPort = origReady, origStop })
	return &stopped
}

// An attached session holds no process handle, so /stop used to be a no-op and an
// orphaned server could never be stopped through the CLI.
func TestStop_AttachedSessionStopsTheListeningServer(t *testing.T) {
	stopped := stubStopPort(t, true)
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.checkPort = 8091

	if err := m.stopThunderID(true); err != nil {
		t.Fatalf("stopThunderID: %v", err)
	}
	if len(*stopped) != 1 || (*stopped)[0] != 8091 {
		t.Fatalf("expected the session port to be stopped, got %v", *stopped)
	}
}

// The fallback must not terminate an unrelated process that happens to hold the port.
func TestStop_AttachedSessionLeavesForeignListenerAlone(t *testing.T) {
	stopped := stubStopPort(t, false)
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.checkPort = 8091

	if err := m.stopThunderID(true); err != nil {
		t.Fatalf("stopThunderID: %v", err)
	}
	if len(*stopped) != 0 {
		t.Fatalf("expected no stop attempt when the product does not answer, got %v", *stopped)
	}
}

// Exiting the CLI stops only what it started: an instance it merely attached to keeps
// running, so Ctrl+C in an onlooker session does not take the server down.
func TestExit_AttachedSessionLeavesTheServerRunning(t *testing.T) {
	stopped := stubStopPort(t, true)
	m := NewReplModel("1.0.0", nil, "/tmp/x", false, false, nil)
	m.checkPort = 8091

	m.killThunderID()

	if len(*stopped) != 0 {
		t.Fatalf("exiting an attached session must not stop the server, got %v", *stopped)
	}
}

// When the CLI owns the launcher it signals that handle instead of the port, so
// start.sh's cleanup trap runs.
func TestStop_OwnedProcessIsSignalledNotPortKilled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX signal")
	}
	stopped := stubStopPort(t, true)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	m := NewReplModel("1.0.0", cmd, "/tmp/x", false, false, nil)
	m.checkPort = 8091

	if err := m.stopThunderID(true); err != nil {
		t.Fatalf("stopThunderID: %v", err)
	}
	if len(*stopped) != 0 {
		t.Fatalf("an owned launcher must be signaled, not port-killed, got %v", *stopped)
	}
}

// The launcher can exit before /stop runs — start.sh's trap only stops the server it
// backgrounded when the trap actually runs, so an undeliverable SIGTERM must not be
// reported as a successful stop.
func TestStop_UndeliverableSignalFallsBackToThePort(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX signal")
	}
	stopped := stubStopPort(t, true)
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}

	m := NewReplModel("1.0.0", cmd, "/tmp/x", false, false, nil)
	m.checkPort = 8091

	if err := m.stopThunderID(true); err != nil {
		t.Fatalf("stopThunderID: %v", err)
	}
	if len(*stopped) != 1 || (*stopped)[0] != 8091 {
		t.Fatalf("expected the port stop to take over, got %v", *stopped)
	}
}

// Exiting is not an explicit stop: the server an exited launcher left behind is not
// this session's to take down on the way out.
func TestExit_UndeliverableSignalLeavesThePortAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX signal")
	}
	stopped := stubStopPort(t, true)
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}

	m := NewReplModel("1.0.0", cmd, "/tmp/x", false, false, nil)
	m.checkPort = 8091

	m.killThunderID()

	if len(*stopped) != 0 {
		t.Fatalf("exiting must not port-stop, got %v", *stopped)
	}
}
