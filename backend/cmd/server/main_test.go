// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/log"
)

func TestRegisterStaticFileHandlers(t *testing.T) {
	logger := log.GetLogger()
	tmpDir := t.TempDir()

	// Create gate and console directories
	gateDir := filepath.Join(tmpDir, "apps", "gate")
	consoleDir := filepath.Join(tmpDir, "apps", "console")
	err := os.MkdirAll(gateDir, 0o750)
	assert.NoError(t, err)
	err = os.MkdirAll(consoleDir, 0o750)
	assert.NoError(t, err)

	// Create index.html files
	requireWriteFile(t, filepath.Join(gateDir, "index.html"), []byte("gate app"))
	requireWriteFile(t, filepath.Join(consoleDir, "index.html"), []byte("console app"))

	t.Run("registers handlers for existing directories", func(t *testing.T) {
		mux := http.NewServeMux()
		registerStaticFileHandlers(context.Background(), logger, mux, tmpDir)

		// Test gate handler
		req := httptest.NewRequest(http.MethodGet, "/gate/", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "gate app")

		// Test console handler
		req = httptest.NewRequest(http.MethodGet, "/console/", nil)
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "console app")
	})

	t.Run("serves js files as application/javascript", func(t *testing.T) {
		jsContent := []byte("console.log('hello');")
		requireWriteFile(t, filepath.Join(gateDir, "app.js"), jsContent)

		mux := http.NewServeMux()
		registerStaticFileHandlers(context.Background(), logger, mux, tmpDir)

		req := httptest.NewRequest(http.MethodGet, "/gate/app.js", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/javascript; charset=utf-8", rr.Header().Get("Content-Type"))
	})

	t.Run("serves console js files as application/javascript", func(t *testing.T) {
		jsContent := []byte("console.log('hello');")
		requireWriteFile(t, filepath.Join(consoleDir, "app.js"), jsContent)

		mux := http.NewServeMux()
		registerStaticFileHandlers(context.Background(), logger, mux, tmpDir)

		req := httptest.NewRequest(http.MethodGet, "/console/app.js", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/javascript; charset=utf-8", rr.Header().Get("Content-Type"))
	})

	t.Run("serves mjs files as application/javascript", func(t *testing.T) {
		mjsContent := []byte("export default {};")
		requireWriteFile(t, filepath.Join(gateDir, "app.mjs"), mjsContent)

		mux := http.NewServeMux()
		registerStaticFileHandlers(context.Background(), logger, mux, tmpDir)

		req := httptest.NewRequest(http.MethodGet, "/gate/app.mjs", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/javascript; charset=utf-8", rr.Header().Get("Content-Type"))
	})

	t.Run("handles missing directories gracefully", func(t *testing.T) {
		emptyTmpDir := t.TempDir()
		mux := http.NewServeMux()
		// Should not panic
		registerStaticFileHandlers(context.Background(), logger, mux, emptyTmpDir)
	})
}

// requireWriteFile writes a file for a test, failing it if the write does not succeed.
func requireWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	cleanPath := filepath.Clean(path)

	if err := os.WriteFile(cleanPath, content, 0o600); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
