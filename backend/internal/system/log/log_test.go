/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package log

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	sysContext "github.com/thunder-id/thunderid/internal/system/context"
)

type LogTestSuite struct {
	suite.Suite
	originalStdout *os.File
	buffer         *bytes.Buffer
}

func TestLogSuite(t *testing.T) {
	suite.Run(t, new(LogTestSuite))
}

func (suite *LogTestSuite) SetupTest() {
	// Capture stdout
	suite.originalStdout = os.Stdout
	suite.buffer = &bytes.Buffer{}
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Start a goroutine to read from the pipe
	go func() {
		if _, err := io.Copy(suite.buffer, r); err != nil {
			suite.T().Errorf("Failed to copy from pipe: %v", err)
		}
	}()
}

func (suite *LogTestSuite) TearDownTest() {
	// Restore stdout
	os.Stdout = suite.originalStdout

	// Reset logger singleton for next test
	logger = nil
	once = sync.Once{}
}

func (suite *LogTestSuite) TestInitLoggerUsesDefaultLevel() {
	logger = nil
	once = sync.Once{}

	assert.NotPanics(suite.T(), func() {
		_ = GetLogger()
	})
	// The logger boots at the default level (info), so debug is not enabled.
	assert.False(suite.T(), GetLogger().IsDebugEnabled())
}

func (suite *LogTestSuite) TestSetLevel() {
	logger = nil
	once = sync.Once{}
	log := GetLogger()

	err := log.SetLevel("debug")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), log.IsDebugEnabled())

	err = log.SetLevel("error")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), log.IsDebugEnabled())

	// An invalid level returns an error and leaves the level unchanged.
	err = log.SetLevel("bogus")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), log.IsDebugEnabled())
}

func (suite *LogTestSuite) TestParseLogLevel() {
	testCases := []struct {
		name      string
		logLevel  string
		expected  slog.Level
		expectErr bool
	}{
		{"Debug", "debug", slog.LevelDebug, false},
		{"Info", "info", slog.LevelInfo, false},
		{"Warn", "warn", slog.LevelWarn, false},
		{"Error", "error", slog.LevelError, false},
		{"Invalid", "invalid", slog.LevelError, true},
		{"Empty", "", slog.LevelInfo, true},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			level, err := parseLogLevel(tc.logLevel)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, level)
			}
		})
	}
}

func (suite *LogTestSuite) TestLogMethods() {
	var buf bytes.Buffer

	logger = nil
	once = sync.Once{}

	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	logHandler := slog.NewTextHandler(&buf, handlerOptions)
	logger = &Logger{
		internal: slog.New(logHandler),
	}
	log := logger

	ctx := context.Background()
	log.Debug(ctx, "Debug message", Field{Key: "test", Value: "debug"})
	log.Info(ctx, "Info message", Field{Key: "test", Value: "info"})
	log.Warn(ctx, "Warning message", Field{Key: "test", Value: "warn"})
	log.Error(ctx, "Error message", Field{Key: "test", Value: "error"})

	output := buf.String()
	assert.Contains(suite.T(), output, "Debug message")
	assert.Contains(suite.T(), output, "Info message")
	assert.Contains(suite.T(), output, "Warning message")
	assert.Contains(suite.T(), output, "Error message")

	assert.Contains(suite.T(), output, "test=debug")
	assert.Contains(suite.T(), output, "test=info")
	assert.Contains(suite.T(), output, "test=warn")
	assert.Contains(suite.T(), output, "test=error")
}

func (suite *LogTestSuite) TestLoggerWith() {
	var buf bytes.Buffer

	logger = nil
	once = sync.Once{}

	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	logHandler := slog.NewTextHandler(&buf, handlerOptions)
	logger = &Logger{
		internal: slog.New(logHandler),
	}
	log := logger

	contextLogger := log.With(Field{Key: "context", Value: "test"})
	assert.NotNil(suite.T(), contextLogger)

	contextLogger.Info(context.Background(), "Context log message")

	output := buf.String()
	assert.Contains(suite.T(), output, "context=test")
	assert.Contains(suite.T(), output, "Context log message")
}

func (suite *LogTestSuite) TestMaskString() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty", "", ""},
		{"Short", "ab", "**"},
		{"ThreeChars", "abc", "***"},
		{"Normal", "password", "p******d"},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := maskString(tc.input)
			assert.Equal(t, tc.expected, result)

			if len(tc.input) > 0 {
				// Should mask the middle characters only
				if len(tc.input) > 3 {
					assert.Equal(t, string(tc.input[0]), string(result[0]), "First character should not be masked")
					assert.Equal(t, string(tc.input[len(tc.input)-1]), string(result[len(result)-1]),
						"Last character should not be masked")

					// All other characters should be masked
					for i := 1; i < len(result)-1; i++ {
						assert.Equal(t, '*', rune(result[i]), "Middle character should be masked")
					}
				}
			}
		})
	}
}

func (suite *LogTestSuite) TestMaskedString() {
	testCases := []struct {
		name     string
		key      string
		input    string
		expected string
	}{
		{"Empty", LoggerKeyUserID, "", ""},
		{"Short", LoggerKeyUserID, "ab", "**"},
		{"UUID", LoggerKeyUserID, "019d3279-78bc-7af0-8ea8-979a9c9a8cb7", "0**********************************7"},
		{"CustomKey", "email", "user@example.com", "u**************m"},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			field := MaskedString(tc.key, tc.input)
			assert.Equal(t, tc.key, field.Key)
			assert.Equal(t, tc.expected, field.Value)
		})
	}
}

func (suite *LogTestSuite) TestMaskedStrings() {
	suite.T().Run("MasksEachEntry", func(t *testing.T) {
		field := MaskedStrings("ids", []string{
			"019d3279-78bc-7af0-8ea8-979a9c9a8cb7",
			"ab",
			"user@example.com",
		})
		assert.Equal(t, "ids", field.Key)
		got, ok := field.Value.([]string)
		assert.True(t, ok, "Value should be a []string")
		assert.Equal(t, []string{
			"0**********************************7",
			"**",
			"u**************m",
		}, got)
	})

	suite.T().Run("EmptySlice", func(t *testing.T) {
		field := MaskedStrings("ids", []string{})
		got, ok := field.Value.([]string)
		assert.True(t, ok)
		assert.Empty(t, got)
	})

	suite.T().Run("DoesNotMutateInput", func(t *testing.T) {
		input := []string{"019d3279-78bc-7af0-8ea8-979a9c9a8cb7"}
		MaskedStrings("ids", input)
		assert.Equal(t, "019d3279-78bc-7af0-8ea8-979a9c9a8cb7", input[0])
	})
}

func (suite *LogTestSuite) TestMaskedMap() {
	suite.T().Run("MasksStringsAndReplacesNonStrings", func(t *testing.T) {
		input := map[string]any{
			"email":  "alice@example.com",
			"short":  "ab",
			"count":  42,
			"active": true,
		}
		field := MaskedMap("filters", input)

		assert.Equal(t, "filters", field.Key)
		got, ok := field.Value.(map[string]any)
		assert.True(t, ok, "Value should be a map[string]any")
		assert.Equal(t, "a***************m", got["email"])
		assert.Equal(t, "**", got["short"])
		assert.Equal(t, "***", got["count"])
		assert.Equal(t, "***", got["active"])
	})

	suite.T().Run("EmptyMap", func(t *testing.T) {
		field := MaskedMap("filters", map[string]any{})
		got, ok := field.Value.(map[string]any)
		assert.True(t, ok)
		assert.Empty(t, got)
	})

	suite.T().Run("DoesNotMutateInput", func(t *testing.T) {
		input := map[string]any{"email": "alice@example.com"}
		MaskedMap("filters", input)
		assert.Equal(t, "alice@example.com", input["email"])
	})
}

// newContextTestLogger creates a Logger backed by a contextHandler writing to buf.
func newContextTestLogger(buf *bytes.Buffer) *Logger {
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	return &Logger{
		internal: slog.New(&contextHandler{Handler: slog.NewTextHandler(buf, handlerOptions)}),
	}
}

func (suite *LogTestSuite) TestContextLogMethodsWithTraceID() {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf)

	ctx := sysContext.WithTraceID(context.Background(), "test-trace-123")

	log.Debug(ctx, "Debug message", Field{Key: "test", Value: "debug"})
	log.Info(ctx, "Info message", Field{Key: "test", Value: "info"})
	log.Warn(ctx, "Warning message", Field{Key: "test", Value: "warn"})
	log.Error(ctx, "Error message", Field{Key: "test", Value: "error"})

	output := buf.String()
	assert.Contains(suite.T(), output, "Debug message")
	assert.Contains(suite.T(), output, "Info message")
	assert.Contains(suite.T(), output, "Warning message")
	assert.Contains(suite.T(), output, "Error message")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Equal(suite.T(), 4, len(lines))
	for _, line := range lines {
		assert.Contains(suite.T(), string(line), LoggerKeyTraceID+"=test-trace-123")
	}
}

func (suite *LogTestSuite) TestContextLogMethodsWithoutTraceID() {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf)

	ctx := context.Background()

	log.Debug(ctx, "Debug message")
	log.Info(ctx, "Info message")
	log.Warn(ctx, "Warning message")
	log.Error(ctx, "Error message")

	output := buf.String()
	assert.Contains(suite.T(), output, "Info message")
	assert.NotContains(suite.T(), output, LoggerKeyTraceID+"=")
}

func (suite *LogTestSuite) TestContextHandlerPreservedByWith() {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf)

	ctx := sysContext.WithTraceID(context.Background(), "test-trace-456")

	derivedLogger := log.With(Field{Key: "component", Value: "TestComponent"})
	derivedLogger.Info(ctx, "Derived log message")

	output := buf.String()
	assert.Contains(suite.T(), output, "Derived log message")
	assert.Contains(suite.T(), output, "component=TestComponent")
	assert.Contains(suite.T(), output, LoggerKeyTraceID+"=test-trace-456")
}

func (suite *LogTestSuite) TestContextHandlerPreservedByWithGroup() {
	var buf bytes.Buffer
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	internal := slog.New(&contextHandler{Handler: slog.NewTextHandler(&buf, handlerOptions)})

	ctx := sysContext.WithTraceID(context.Background(), "test-trace-789")

	internal.WithGroup("group").InfoContext(ctx, "Grouped log message", slog.String("key", "value"))

	output := buf.String()
	assert.Contains(suite.T(), output, "Grouped log message")
	assert.Contains(suite.T(), output, "group.key=value")
	assert.Contains(suite.T(), output, LoggerKeyTraceID+"=test-trace-789")
}

func (suite *LogTestSuite) TestGetLoggerUsesContextHandler() {
	logger = nil
	once = sync.Once{}

	log := GetLogger()
	_, ok := log.internal.Handler().(*contextHandler)
	assert.True(suite.T(), ok, "GetLogger should wrap the handler with contextHandler")
}

func (suite *LogTestSuite) TestServerErrorWriterWrite() {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf)
	w := &serverErrorWriter{logger: log}

	input := []byte("http: TLS handshake error from 127.0.0.1:56960: remote error: tls: unknown certificate\n")
	n, err := w.Write(input)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), len(input), n)

	output := buf.String()
	assert.Contains(suite.T(), output, "level=WARN")
	assert.Contains(suite.T(), output, "http: TLS handshake error from 127.0.0.1:56960")
	// The trailing newline should be trimmed from the logged message.
	assert.NotContains(suite.T(), output, "certificate\\n")
}

func (suite *LogTestSuite) TestServerErrorWriterRespectsLevel() {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf)
	log.levelVar = new(slog.LevelVar)
	log.levelVar.Set(slog.LevelError)
	log.internal = slog.New(&contextHandler{Handler: slog.NewTextHandler(&buf,
		&slog.HandlerOptions{Level: log.levelVar})})
	w := &serverErrorWriter{logger: log}

	_, err := w.Write([]byte("some server error"))

	assert.NoError(suite.T(), err)
	// WARN is below the ERROR threshold, so nothing should be emitted.
	assert.Empty(suite.T(), buf.String())
}

func (suite *LogTestSuite) TestNewServerErrorLog() {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf)

	errorLog := NewServerErrorLog(log)
	assert.NotNil(suite.T(), errorLog)

	errorLog.Print("connection reset by peer")

	output := buf.String()
	assert.Contains(suite.T(), output, "level=WARN")
	assert.Contains(suite.T(), output, "connection reset by peer")
}

func (suite *LogTestSuite) TestConvertFields() {
	fields := []Field{
		{Key: "string", Value: "value"},
		{Key: "int", Value: 42},
		{Key: "bool", Value: true},
	}

	attrs := convertFields(fields)
	assert.Equal(suite.T(), 3, len(attrs))

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	testLogger := slog.New(handler)

	testLogger.Info("test", attrs...)

	output := buf.String()
	assert.Contains(suite.T(), output, "string=value")
	assert.Contains(suite.T(), output, "int=42")
	assert.Contains(suite.T(), output, "bool=true")
}
