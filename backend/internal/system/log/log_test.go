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
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/constants"
)

type LogTestSuite struct {
	suite.Suite
	originalLogLevel string
	originalStdout   *os.File
	buffer           *bytes.Buffer
}

func TestLogSuite(t *testing.T) {
	suite.Run(t, new(LogTestSuite))
}

func (suite *LogTestSuite) SetupTest() {
	// Save original environment variable
	suite.originalLogLevel = os.Getenv(constants.LogLevelEnvironmentVariable)

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
	// Restore original environment variable
	err := os.Setenv(constants.LogLevelEnvironmentVariable, suite.originalLogLevel)
	if err != nil {
		suite.T().Errorf("Failed to restore environment variable: %v", err)
	}

	// Restore stdout
	os.Stdout = suite.originalStdout

	// Reset logger singleton for next test
	logger = nil
	once = sync.Once{}
}

func (suite *LogTestSuite) TestInitLoggerWithEnvironmentVariable() {
	testCases := []struct {
		name     string
		logLevel string
		isValid  bool
	}{
		{"DefaultLevel", "", true},
		{"DebugLevel", "debug", true},
		{"InfoLevel", "info", true},
		{"WarnLevel", "warn", true},
		{"ErrorLevel", "error", true},
		{"InvalidLevel", "unknown", false},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			logger = nil
			once = sync.Once{}

			if tc.logLevel != "" {
				err := os.Setenv(constants.LogLevelEnvironmentVariable, tc.logLevel)
				assert.NoError(t, err)
			} else {
				err := os.Unsetenv(constants.LogLevelEnvironmentVariable)
				assert.NoError(t, err)
			}

			if tc.isValid {
				assert.NotPanics(t, func() {
					_ = GetLogger()
				})
			} else {
				assert.Panics(t, func() {
					_ = GetLogger()
				})
			}
		})
	}
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

	err := os.Setenv(constants.LogLevelEnvironmentVariable, "debug")
	assert.NoError(suite.T(), err)

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

	log.Debug("Debug message", Field{Key: "test", Value: "debug"})
	log.Info("Info message", Field{Key: "test", Value: "info"})
	log.Warn("Warning message", Field{Key: "test", Value: "warn"})
	log.Error("Error message", Field{Key: "test", Value: "error"})

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

	contextLogger.Info("Context log message")

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
