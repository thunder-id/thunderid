// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"io"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
