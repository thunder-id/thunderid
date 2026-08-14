// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/thunder-id/thunderid/tools/cli/internal/services/setup"
)

// withTerminalWidth renders at a fixed width for the duration of the test.
func withTerminalWidth(t *testing.T, width int) {
	t.Helper()
	orig := terminalWidth
	terminalWidth = func() int { return width }
	t.Cleanup(func() { terminalWidth = orig })
}

// The ASCII art is a fixed width, so on a narrow terminal the box used to overflow and
// every line wrapped. Below that width the banner must fall back to a compact form.
func TestBannerString_CompactOnNarrowTerminal(t *testing.T) {
	withTerminalWidth(t, 40)

	out := BannerString()

	if strings.Contains(out, `|_   _| |`) {
		t.Fatalf("expected the compact banner on a 40-column terminal:\n%s", out)
	}
	if w := lipgloss.Width(out); w > 40 {
		t.Fatalf("banner is %d columns wide, does not fit 40:\n%s", w, out)
	}
}

func TestBannerString_ArtOnWideTerminal(t *testing.T) {
	withTerminalWidth(t, 120)

	out := BannerString()

	if !strings.Contains(out, `|_   _| |`) {
		t.Fatalf("expected the full banner art on a wide terminal:\n%s", out)
	}
}

// Boxes have to stay inside the terminal: a long message wraps instead of pushing the
// border past the right edge.
func TestBoxes_FitNarrowTerminal(t *testing.T) {
	withTerminalWidth(t, 48)
	long := strings.Repeat("a very long failure message ", 6)

	out := captureStdout(t, func() { Fatal(long) })

	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := lipgloss.Width(line); w > 48 {
			t.Fatalf("line is %d columns wide, does not fit 48: %q", w, line)
		}
	}
}

// A scripted or CI run has no terminal on stdin, so prompts must be skipped there
// rather than asking a question nothing can answer.
func TestInteractive_FalseWithoutATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close() //nolint:errcheck
	defer w.Close() //nolint:errcheck
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig })

	if Interactive() {
		t.Fatal("expected a pipe on stdin not to count as a terminal")
	}
}

// The prompt has to name what it would stop, so the user does not kill an unrelated app.
func TestPromptPortConflict_ListsHolders(t *testing.T) {
	out := holderLines([]setup.PortHolder{
		{Port: 8090, PID: 61201, Name: "thunderid"},
		{Port: 8090, PID: 61202},
	})

	for _, want := range []string{"8090", "thunderid", "61201", "61202"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in the holder lines:\n%s", want, out)
		}
	}
	if lines := strings.Split(out, "\n"); len(lines) != 2 {
		t.Fatalf("expected one line per holder, got %d:\n%s", len(lines), out)
	}
}

// Setup seeds the console application's redirect URIs with the port it ran on, so a run
// that does not re-run setup must not be able to move the port.
func TestPortConflictOptions_OmitsAlternateWithoutOne(t *testing.T) {
	options := portConflictOptions(8090, 0)

	if len(options) != 2 {
		t.Fatalf("expected kill and abort only, got %d options", len(options))
	}
	for _, opt := range options {
		if opt.Value == UseAlternatePort {
			t.Fatalf("expected no alternate-port option, got %q", opt.Key)
		}
	}
}

// Setup runs in this invocation, so the seeded port is whatever is picked here.
func TestPortConflictOptions_OffersAlternateWhenGiven(t *testing.T) {
	options := portConflictOptions(8090, 8091)

	var alt *huh.Option[PortConflictChoice]
	for i := range options {
		if options[i].Value == UseAlternatePort {
			alt = &options[i]
		}
	}
	if alt == nil {
		t.Fatalf("expected an alternate-port option in %d options", len(options))
	}
	if !strings.Contains(alt.Key, "8091") {
		t.Fatalf("expected the alternate port in the option label, got %q", alt.Key)
	}
}

// A scripted run cannot answer the prompt, and reporting a cancellation nobody chose
// would abort the sample. It continues instead, like the product-port path does.
func TestConfirmStopPortHolders_ContinuesWithoutATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close() //nolint:errcheck
	defer w.Close() //nolint:errcheck
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig })

	var confirmed bool
	captureStdout(t, func() {
		confirmed = ConfirmStopPortHolders([]setup.PortHolder{{Port: 5173, PID: 61201, Name: "node"}})
	})

	if !confirmed {
		t.Fatal("expected a non-interactive run to continue instead of canceling")
	}
}

func TestHolderLines_EmptyForNoHolders(t *testing.T) {
	if out := holderLines(nil); out != "" {
		t.Fatalf("expected no output for no holders, got %q", out)
	}
}
