// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
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

// TestConfigureAppliesToLoggerDerivedBeforeConfigure is the regression test for the
// reported defect: services cache a derived logger in their constructor, which used to
// pin them to the boot text handler when Configure ran afterwards.
func TestConfigureAppliesToLoggerDerivedBeforeConfigure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	svc := log.With(String(LoggerKeyComponentName, "XService"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	svc.Info(context.Background(), "derived before configure")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(content), `"msg":"derived before configure"`)
	assert.Contains(t, string(content), `"`+LoggerKeyComponentName+`":"XService"`)
}

func TestConfigureAppliesToNestedDerivations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	nested := log.With(String(LoggerKeyComponentName, "XService")).
		With(String("sub", "worker")).
		WithTraceID("trace-nested")

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	nested.Info(context.Background(), "nested message")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(content), `"msg":"nested message"`)
	assert.Contains(t, string(content), `"`+LoggerKeyComponentName+`":"XService"`)
	assert.Contains(t, string(content), `"sub":"worker"`)
	assert.Contains(t, string(content), `"`+LoggerKeyTraceID+`":"trace-nested"`)
}

func TestDerivedLoggerFollowsSecondConfigure(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.log")
	second := filepath.Join(dir, "second.log")
	log := freshLogger()

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		File:        rollingfile.Config{Path: first},
	}))

	svc := log.With(String(LoggerKeyComponentName, "XService"))
	svc.Info(context.Background(), "before reconfigure")

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: second},
	}))
	defer func() { _ = log.Close() }()

	svc.Info(context.Background(), "after reconfigure")

	firstContent, err := os.ReadFile(first) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(firstContent), "before reconfigure")
	assert.NotContains(t, string(firstContent), "after reconfigure")

	secondContent, err := os.ReadFile(second) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(secondContent), `"msg":"after reconfigure"`)
	assert.NotContains(t, string(secondContent), "before reconfigure")
}

// TestDerivationOrderPreservedAcrossConfigure pins the slog contract that WithAttrs and
// WithGroup order is significant: attributes added before a group stay at the top level,
// while attributes added after it (and the record's own) nest inside the group.
func TestDerivationOrderPreservedAcrossConfigure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	derived := &Logger{
		internal: slog.New(log.internal.Handler().
			WithAttrs([]slog.Attr{slog.String("top", "1")}).
			WithGroup("g").
			WithAttrs([]slog.Attr{slog.String("inner", "2")})),
		root: log.root,
	}

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	derived.Info(context.Background(), "ordered", String("rec", "3"))

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal(content, &record))
	assert.Equal(t, "1", record["top"])
	group, ok := record["g"].(map[string]any)
	require.True(t, ok, "attributes after WithGroup must be nested under the group")
	assert.Equal(t, "2", group["inner"])
	assert.Equal(t, "3", group["rec"])
}

// TestSiblingDerivationsDoNotAlias guards the defensive slice copy in derive: two loggers
// derived from the same parent must not share a backing array.
func TestSiblingDerivationsDoNotAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	parent := log.With(String(LoggerKeyComponentName, "XService"))
	first := parent.With(String("k", "1"))
	second := parent.With(String("k", "2"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	first.Info(context.Background(), "first message")
	second.Info(context.Background(), "second message")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"k":"1"`)
	assert.NotContains(t, lines[0], `"k":"2"`)
	assert.Contains(t, lines[1], `"k":"2"`)
	assert.NotContains(t, lines[1], `"k":"1"`)
}

func TestSetLevelAffectsDerivedLoggersAfterConfigure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	svc := log.With(String(LoggerKeyComponentName, "XService"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	require.NoError(t, log.SetLevel("error"))
	svc.Info(context.Background(), "suppressed message")

	require.NoError(t, log.SetLevel("debug"))
	svc.Debug(context.Background(), "emitted message")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.NotContains(t, string(content), "suppressed message")
	assert.Contains(t, string(content), "emitted message")
}

func TestTraceIDPreservedOnDerivedLoggerAfterConfigure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	svc := log.With(String(LoggerKeyComponentName, "XService"))

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	svc.Info(sysContext.WithTraceID(context.Background(), "trace-abc"), "traced message")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(content), `"`+LoggerKeyTraceID+`":"trace-abc"`)
}

// TestConfigureConcurrentWithLogging exercises the atomic state swap against loggers
// derived up front, the way the access log middleware holds one across a reconfigure.
func TestConfigureConcurrentWithLogging(t *testing.T) {
	dir := t.TempDir()
	log := freshLogger()

	derived := make([]*Logger, 8)
	for i := range derived {
		derived[i] = log.With(String(LoggerKeyComponentName, "Service"+strconv.Itoa(i)))
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for _, l := range derived {
		wg.Add(1)
		go func(l *Logger) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					l.Info(context.Background(), "concurrent message")
				}
			}
		}(l)
	}

	for i := range 10 {
		format := "text"
		if i%2 == 0 {
			format = "json"
		}
		require.NoError(t, log.Configure(OutputOptions{
			FileEnabled: true,
			Format:      format,
			File:        rollingfile.Config{Path: filepath.Join(dir, "run"+strconv.Itoa(i)+".log")},
		}))
	}

	close(stop)
	wg.Wait()
	assert.NoError(t, log.Close())
}

func TestCloseIsSharedAndIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		File:        rollingfile.Config{Path: path},
	}))

	derived := log.With(String(LoggerKeyComponentName, "XService"))
	require.NoError(t, derived.Close())
	assert.Nil(t, log.root.fileWriter, "closing a derived logger closes the shared writer")
	assert.NoError(t, log.Close(), "Close must be idempotent")
}

func BenchmarkDerivedLoggerInfo(b *testing.B) {
	path := filepath.Join(b.TempDir(), "thunderid.log")
	log := freshLogger()

	if err := log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      "json",
		File:        rollingfile.Config{Path: path},
	}); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = log.Close() }()

	svc := log.With(String(LoggerKeyComponentName, "XService"))
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		svc.Info(ctx, "benchmark message")
	}
}

// TestSetFormatPreservesFileOutput is the regression test for the embedded-engine
// defect: the engine applied its configured format by calling Configure with a
// console-only OutputOptions, which replaced the host's output and closed its file
// writer. SetFormat changes only the format.
func TestSetFormatPreservesFileOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	require.NoError(t, log.Configure(OutputOptions{
		ConsoleEnabled: false,
		FileEnabled:    true,
		Format:         formatText,
		File:           rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	log.Info(context.Background(), "before format change")
	require.NoError(t, log.SetFormat(formatJSON))
	log.Info(context.Background(), "after format change")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 2, "file output must survive SetFormat")
	assert.Contains(t, lines[0], "before format change")
	assert.NotContains(t, lines[0], "{", "first record predates the format change")

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &record),
		"record after SetFormat must be JSON")
	assert.Equal(t, "after format change", record["msg"])
}

// TestSetFormatAppliesToLoggerDerivedBeforeCall ensures the format swap reaches loggers
// that services captured in their constructors.
func TestSetFormatAppliesToLoggerDerivedBeforeCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatText,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	derived := log.With(String(LoggerKeyComponentName, "FlowEngine"))
	require.NoError(t, log.SetFormat(formatJSON))
	derived.Info(context.Background(), "executing node")

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(content))), &record))
	assert.Equal(t, "executing node", record["msg"])
	assert.Equal(t, "FlowEngine", record[LoggerKeyComponentName])
}

func TestSetFormatRejectsInvalidFormats(t *testing.T) {
	log := freshLogger()

	assert.Error(t, log.SetFormat(""), "empty format must be rejected")
	assert.Error(t, log.SetFormat("xml"), "unsupported format must be rejected")
	assert.NoError(t, log.SetFormat("JSON"), "format matching is case-insensitive")
}

// TestSetFormatDoesNotCloseFileWriter guards the specific mechanism that broke the
// embedded path: Configure closes the previous file writer, so a format-only change
// must not go through it.
func TestSetFormatDoesNotCloseFileWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	log := freshLogger()

	require.NoError(t, log.Configure(OutputOptions{
		FileEnabled: true,
		Format:      formatText,
		File:        rollingfile.Config{Path: path},
	}))
	defer func() { _ = log.Close() }()

	writer := log.root.fileWriter
	require.NotNil(t, writer)

	require.NoError(t, log.SetFormat(formatJSON))
	assert.Same(t, writer, log.root.fileWriter, "SetFormat must not swap the file writer")
}
