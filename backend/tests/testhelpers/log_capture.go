// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testhelpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
)

// CaptureLogs sends the shared logger's output to a file at the given level while fn runs, and
// returns what it wrote. The console output and the INFO level are restored afterwards.
func CaptureLogs(t *testing.T, level string, fn func()) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.log")
	logger := log.GetLogger()
	require.NoError(t, logger.Configure(log.OutputOptions{FileEnabled: true, File: rollingfile.Config{Path: path}}))
	require.NoError(t, logger.SetLevel(level))
	defer func() {
		require.NoError(t, logger.SetLevel("INFO"))
		require.NoError(t, logger.Configure(log.OutputOptions{ConsoleEnabled: true}))
	}()
	fn()
	// #nosec G304 -- path is in the test's temp directory
	out, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(out)
}
