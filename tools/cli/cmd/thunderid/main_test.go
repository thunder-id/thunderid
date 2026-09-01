// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}

	return string(out)
}

func TestVersionLineUsesInjectedVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	version = "1.2.3"

	if got, want := versionLine(), "thunderid 1.2.3"; got != want {
		t.Errorf("versionLine() = %q, want %q", got, want)
	}
}

func TestVersionDefaultsToPlaceholder(t *testing.T) {
	const placeholder = "0.0.0-semantically-released"

	if version != placeholder {
		t.Errorf("version = %q, want %q when no value is injected at build time", version, placeholder)
	}
}

func TestHasVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "no args", args: nil, want: false},
		{name: "long form", args: []string{"--version"}, want: true},
		{name: "short form", args: []string{"-V"}, want: true},
		{name: "after another flag", args: []string{"--verbose", "--version"}, want: true},
		{name: "after a command", args: []string{"upgrade", "--version"}, want: true},
		{name: "after a command with argument", args: []string{"try", "react", "-V"}, want: true},
		{name: "verbose short form is not version", args: []string{"-v"}, want: false},
		{name: "unrelated flags only", args: []string{"--verbose", "--setup"}, want: false},
		{name: "command only", args: []string{"upgrade"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasVersionFlag(tt.args); got != tt.want {
				t.Errorf("hasVersionFlag(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestPrintUsageListsVersionFlag(t *testing.T) {
	usage := captureStdout(t, printUsage)

	if !strings.Contains(usage, "--version, -V") {
		t.Errorf("printUsage() output does not list the --version flag:\n%s", usage)
	}
}
