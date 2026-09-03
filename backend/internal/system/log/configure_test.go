// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
)

// freshLogger resets the singleton and returns a new logger instance.
func freshLogger() *Logger {
	logger = nil
	once = sync.Once{}
	return GetLogger()
}

func TestConfigureWritesToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "thunderid.log")
	log := freshLogger()

	err := log.Configure(OutputOptions{
		FileEnabled: true,
		File:        rollingfile.Config{Path: path},
	})
	require.NoError(t, err)
	defer func() { _ = log.Close() }()

	ctx := context.WithValue(context.Background(), sysContext.TraceIDKey, "trace-xyz")
	log.Info(ctx, "hello file", String("k", "v"))

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(content), "hello file")
	assert.Contains(t, string(content), "k=v")
	assert.Contains(t, string(content), "trace_id=trace-xyz")
}

func TestConfigureJSONFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	err := log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	})
	require.NoError(t, err)
	defer func() { _ = log.Close() }()

	log.Info(context.Background(), "json message")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(content), `"msg":"json message"`)
	assert.Contains(t, string(content), `"level":"INFO"`)
}

func TestConfigureFallsBackToStdoutWhenNothingEnabled(t *testing.T) {
	log := freshLogger()

	err := log.Configure(OutputOptions{ConsoleEnabled: false, FileEnabled: false})
	require.NoError(t, err)

	// The logger must remain usable (writing to the stdout fallback).
	assert.NotPanics(t, func() {
		log.Info(context.Background(), "fallback message")
	})
	assert.Nil(t, log.root.fileWriter, "no file writer should be created")
}

func TestConfigureErrorsOnInvalidFilePath(t *testing.T) {
	log := freshLogger()

	err := log.Configure(OutputOptions{
		FileEnabled: true,
		File:        rollingfile.Config{Path: ""},
	})
	assert.Error(t, err)
}

func TestConfigureConsoleAndFileWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	err := log.Configure(OutputOptions{
		ConsoleEnabled: true,
		FileEnabled:    true,
		File:           rollingfile.Config{Path: path},
	})
	require.NoError(t, err)
	defer func() { _ = log.Close() }()

	log.Info(context.Background(), "dual output")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(content), "dual output")
}

// readLines returns the non-empty lines written to the log file at path.
func readLines(t *testing.T, path string) []string {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(content)), "\n")
}

// TestConfigureAppliesFormatToLoggerDerivedBeforeConfigure covers the reported defect:
// components derive their logger at construction time, which in the embedded engine
// happens before the configured format is known. The derived logger must follow the
// format installed later rather than staying on the text handler it booted with.
func TestConfigureAppliesFormatToLoggerDerivedBeforeConfigure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	derived := log.With(String(LoggerKeyComponentName, "FlowEngine"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatJSON,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	derived.Info(context.Background(), "Executing node")

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(readLines(t, path)[0]), &record))
	assert.Equal(t, "Executing node", record["msg"])
	assert.Equal(t, "FlowEngine", record[LoggerKeyComponentName])
}

// TestConfigureKeepsTraceIDOnLoggerDerivedBeforeConfigure guards the context decoration,
// which is replayed onto the swapped handler along with the recorded attributes.
func TestConfigureKeepsTraceIDOnLoggerDerivedBeforeConfigure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	derived := log.With(String(LoggerKeyComponentName, "AuthAssertExecutor"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatJSON,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	ctx := sysContext.WithTraceID(context.Background(), "trace-1")
	derived.Info(ctx, "Generated JWT token for authentication assertion")

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(readLines(t, path)[0]), &record))
	assert.Equal(t, "trace-1", record[LoggerKeyTraceID])
}

// TestConfigureFollowedByReconfigureSwitchesFormatBack proves the fix is not "always
// JSON": every live logger follows each subsequent Configure call.
func TestConfigureFollowedByReconfigureSwitchesFormatBack(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "json.log")
	textPath := filepath.Join(dir, "text.log")
	log := freshLogger()

	derived := log.With(String(LoggerKeyComponentName, "DBProvider"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatJSON,
		File:        rollingfile.Config{Path: jsonPath},
	}))
	derived.Info(context.Background(), "first")

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatText,
		File:        rollingfile.Config{Path: textPath},
	}))
	defer func() { _ = log.Close() }()
	derived.Info(context.Background(), "second")

	assert.Contains(t, readLines(t, jsonPath)[0], `"msg":"first"`)
	assert.Contains(t, readLines(t, textPath)[0], `msg=second`)
}

// TestConfigureKeepsSiblingLoggersIndependent guards the derivation slice copy: siblings
// derived from one parent must not overwrite each other's attributes.
func TestConfigureKeepsSiblingLoggersIndependent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	engine := log.With(String(LoggerKeyComponentName, "FlowEngine"))
	first := engine.With(String("nodeID", "consent_check"))
	second := engine.With(String("nodeID", "auth_assert"))
	grandchild := second.With(String("nodeType", "TASK_EXECUTION"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatJSON,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	first.Info(context.Background(), "one")
	second.Info(context.Background(), "two")
	grandchild.Info(context.Background(), "three")

	lines := readLines(t, path)
	require.Len(t, lines, 3)

	records := make([]map[string]any, len(lines))
	for i, line := range lines {
		require.NoError(t, json.Unmarshal([]byte(line), &records[i]))
	}
	assert.Equal(t, "consent_check", records[0]["nodeID"])
	assert.Equal(t, "auth_assert", records[1]["nodeID"])
	assert.Equal(t, "auth_assert", records[2]["nodeID"])
	assert.Equal(t, "TASK_EXECUTION", records[2]["nodeType"])
	for _, record := range records {
		assert.Equal(t, "FlowEngine", record[LoggerKeyComponentName])
	}
}

// TestConfigurePreservesLevelSetEarlier guards the shared level variable across the
// handler swap.
func TestConfigurePreservesLevelSetEarlier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	require.NoError(t, log.SetLevel("error"))
	derived := log.With(String(LoggerKeyComponentName, "UserInfoHandler"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatJSON,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	derived.Debug(context.Background(), "UserInfo response sent successfully")
	derived.Error(context.Background(), "failed")

	lines := readLines(t, path)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], `"msg":"failed"`)
}

func TestSetFormatChangesFormatWithoutReplacingOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatText,
		File:        rollingfile.Config{Path: path},
	}))

	writer := log.root.fileWriter
	derived := log.With(String(LoggerKeyComponentName, "UserInfoHandler"))

	require.NoError(t, log.SetFormat(formatJSON))
	derived.Info(context.Background(), "UserInfo response sent successfully")

	assert.Same(t, writer, log.root.fileWriter, "SetFormat must keep the configured file writer")
	assert.Contains(t, readLines(t, path)[0], `"msg":"UserInfo response sent successfully"`)

	// Re-applying the same format is a no-op rather than a needless handler rebuild.
	require.NoError(t, log.SetFormat(formatJSON))
	assert.Same(t, writer, log.root.fileWriter)

	// Close releases the file writer once; a second call is a no-op.
	require.NoError(t, log.Close())
	require.NoError(t, log.Close())
}

func TestSetFormatRejectsUnsupportedFormat(t *testing.T) {
	log := freshLogger()

	assert.Error(t, log.SetFormat(""))
	assert.Error(t, log.SetFormat("xml"))
	assert.NoError(t, log.SetFormat("JSON"))
}

// TestConfigureIsSafeUnderConcurrentLogging exercises the atomic state swap against
// in-flight writes; it is meaningful under -race.
func TestConfigureIsSafeUnderConcurrentLogging(t *testing.T) {
	dir := t.TempDir()
	log := freshLogger()
	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatText,
		File:        rollingfile.Config{Path: filepath.Join(dir, "initial.log")},
	}))
	defer func() { _ = log.Close() }()

	derived := log.With(String(LoggerKeyComponentName, "FlowEngine"))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					derived.Info(context.Background(), "concurrent")
				}
			}
		}()
	}

	for i := range 20 {
		format := formatText
		if i%2 == 0 {
			format = formatJSON
		}
		require.NoError(t, log.Configure(OutputOptions{
			FileEnabled: true,
			Format:      format,
			File:        rollingfile.Config{Path: filepath.Join(dir, fmt.Sprintf("swap-%d.log", i))},
		}))
	}

	close(stop)
	wg.Wait()
}

// TestConfigureReplaysGroupsOnDerivedHandler covers the WithGroup derivation path: groups
// recorded before Configure must be replayed, in order, onto the swapped handler.
func TestConfigureReplaysGroupsOnDerivedHandler(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	grouped := slog.New(log.internal.Handler().
		WithAttrs([]slog.Attr{slog.String(LoggerKeyComponentName, "FlowEngine")}).
		WithGroup("node").
		WithAttrs([]slog.Attr{slog.String("nodeID", "auth_assert")}))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatJSON,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	grouped.Info("Executing node")

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(readLines(t, path)[0]), &record))
	assert.Equal(t, "FlowEngine", record[LoggerKeyComponentName])
	node, ok := record["node"].(map[string]any)
	require.True(t, ok, "the group must survive the handler swap: %v", record)
	assert.Equal(t, "auth_assert", node["nodeID"])
}

// TestDynamicHandlerIgnoresEmptyDerivations guards the no-op fast paths: slog treats an
// empty group and an empty attribute list as no-ops, so neither may allocate a new handler.
func TestDynamicHandlerIgnoresEmptyDerivations(t *testing.T) {
	handler := freshLogger().internal.Handler()

	assert.Same(t, handler, handler.WithGroup(""))
	assert.Same(t, handler, handler.WithAttrs(nil))
}
