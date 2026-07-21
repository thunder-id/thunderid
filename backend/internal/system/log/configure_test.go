/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"context"
	"os"
	"path/filepath"
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
	assert.Nil(t, log.fileWriter, "no file writer should be created")
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
