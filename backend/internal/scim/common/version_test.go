// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckIfMatch tests Check If Match.
func TestCheckIfMatch(t *testing.T) {
	const currentVersion = `W/"abc12345"`

	tests := []struct {
		name      string
		ifMatch   string
		wantError bool
	}{
		{"empty header is not a precondition", "", false},
		{"wildcard always matches", "*", false},
		{"exact match", `W/"abc12345"`, false},
		{"match ignoring weak prefix on request side", `"abc12345"`, false},
		{"match with surrounding whitespace", `  W/"abc12345"  `, false},
		{"one of a comma-separated list matches", `W/"deadbeef", W/"abc12345"`, false},
		{"mismatch", `W/"deadbeef"`, true},
		{"comma-separated list with no match", `W/"111", W/"222"`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckIfMatch(tt.ifMatch, currentVersion)
			if tt.wantError {
				require.NotNil(t, err)
				require.Equal(t, ErrorPreconditionFailed.Code, err.Code)
				return
			}
			require.Nil(t, err)
		})
	}
}

// TestNormalizeETag tests Normalize E Tag.
func TestNormalizeETag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `abc123`, "abc123"},
		{"quoted", `"abc123"`, "abc123"},
		{"weak quoted", `W/"abc123"`, "abc123"},
		{"weak quoted with whitespace", ` W/"abc123" `, "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeETag(tt.in))
		})
	}
}

// TestGenerateVersion_MarshalError tests Generate Version for Marshal Error.
func TestGenerateVersion_MarshalError(t *testing.T) {
	// A channel cannot be marshaled to JSON, triggering the error path
	state := make(chan int)
	version := GenerateVersion(state)
	require.Equal(t, `W/"0000000000000000"`, version)
}
