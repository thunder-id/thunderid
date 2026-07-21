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

package rollingfile

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfWindows skips permission-based tests where POSIX mode bits do not apply.
func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on Windows")
	}
}

// makeReadOnlyDir makes dir read-only so filesystem mutations inside it (rename,
// create) fail, and restores it afterwards so the temp dir can be cleaned up.
func makeReadOnlyDir(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.Chmod(dir, 0o500))       // #nosec G302 -- a directory needs its execute bit; test-only.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- restore writable so temp cleanup can remove it.
}

// backups returns the rotated files (excluding the active file) for a writer path.
func backups(t *testing.T, path string) []string {
	t.Helper()
	matches, err := filepath.Glob(path + ".*")
	require.NoError(t, err)
	return matches
}

func TestNewCreatesFileAndDirectoryWithPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	path := filepath.Join(dir, "thunderid.log")

	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(dirPermissions), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(filePermissions), fileInfo.Mode().Perm())
}

func TestNewRequiresPath(t *testing.T) {
	_, err := New(Config{Path: ""})
	assert.Error(t, err)
}

func TestNewClampsNegativeValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: -5, IntervalDays: -1, MaxBackups: -1, MaxAgeDays: -1})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	// A negative size clamps to 0, disabling size rotation.
	_, err = w.Write(make([]byte, 1024))
	require.NoError(t, err)
	assert.Empty(t, backups(t, path))
}

func TestNewErrorWhenPathIsDirectory(t *testing.T) {
	// The path resolves to an existing directory, so opening it as a file fails.
	_, err := New(Config{Path: t.TempDir()})
	assert.Error(t, err)
}

func TestNewErrorWhenDirectoryUncreatable(t *testing.T) {
	skipIfWindows(t)
	parent := t.TempDir()
	makeReadOnlyDir(t, parent)

	_, err := New(Config{Path: filepath.Join(parent, "sub", "thunderid.log")})
	assert.Error(t, err)
}

func TestRotateHandlesRenameFailure(t *testing.T) {
	skipIfWindows(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: 0.01})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	chunk := make([]byte, 6*1024)
	_, err = w.Write(chunk)
	require.NoError(t, err)

	// Make the directory read-only so the rename during rotation fails.
	makeReadOnlyDir(t, dir)

	_, err = w.Write(chunk)
	assert.Error(t, err, "the failed rotation should surface through Write")
	assert.Empty(t, backups(t, path), "no backup should exist when the rename fails")
}

func TestScheduledRotationHandlesFailure(t *testing.T) {
	skipIfWindows(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_, err = w.Write([]byte("content\n"))
	require.NoError(t, err)

	makeReadOnlyDir(t, dir)

	// A failing scheduled rotation must not panic.
	assert.NotPanics(t, func() { w.rotateOnSchedule() })
}

func TestUniqueRotatedPathAvoidsGzCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.Local)
	base := path + "." + now.Format(rotationTimeFormat)
	require.NoError(t, os.WriteFile(base+".gz", []byte("x"), filePermissions))

	got := w.uniqueRotatedPath(now)
	assert.Equal(t, base+".1", got, "a name whose .gz variant exists must be skipped")
}

func TestCompressFileErrorsOnMissingSource(t *testing.T) {
	err := compressFile(filepath.Join(t.TempDir(), "does-not-exist.log"))
	assert.Error(t, err)
}

func TestWriteAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_, err = w.Write([]byte("first\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("second\n"))
	require.NoError(t, err)

	content, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(content))
	assert.Empty(t, backups(t, path))
}

func TestNoRotationWhenSizeDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: 0})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	chunk := make([]byte, 600*1024)
	for i := 0; i < 5; i++ {
		_, err = w.Write(chunk)
		require.NoError(t, err)
	}
	assert.Empty(t, backups(t, path), "no rotation should happen when size is disabled")
}

func TestSizeRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: 1})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	chunk := make([]byte, 600*1024) // two of these exceed 1MB
	_, err = w.Write(chunk)
	require.NoError(t, err)
	assert.Empty(t, backups(t, path), "first write must not rotate an empty file")

	_, err = w.Write(chunk)
	require.NoError(t, err)
	assert.Len(t, backups(t, path), 1, "second write should trigger one rotation")
}

func TestWriteRecoversFromMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	// Simulate a prior rotation failure that left the writer without an open file.
	require.NoError(t, w.file.Close())
	w.file = nil

	// The next write should transparently reopen the file and succeed.
	_, err = w.Write([]byte("after recovery\n"))
	require.NoError(t, err)

	data, err := os.ReadFile(path) // #nosec G304 -- test reads a file under t.TempDir().
	require.NoError(t, err)
	assert.Contains(t, string(data), "after recovery")
}

func TestFractionalSizeRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	// 0.01 MB is ~10 KB, so a couple of small writes should rotate.
	w, err := New(Config{Path: path, MaxSizeMB: 0.01})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	chunk := make([]byte, 6*1024) // two of these exceed ~10 KB
	_, err = w.Write(chunk)
	require.NoError(t, err)
	assert.Empty(t, backups(t, path), "first write must not rotate an empty file")

	_, err = w.Write(chunk)
	require.NoError(t, err)
	assert.Len(t, backups(t, path), 1, "a sub-MB threshold should trigger rotation")
}

func TestRetentionByCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: 1, MaxBackups: 2})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	chunk := make([]byte, 600*1024)
	// Each write after the first triggers a rotation; run enough to exceed MaxBackups.
	for i := 0; i < 6; i++ {
		_, err = w.Write(chunk)
		require.NoError(t, err)
	}
	assert.LessOrEqual(t, len(backups(t, path)), 2, "retention should cap rotated files at MaxBackups")
}

func TestRetentionByAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxAgeDays: 30})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	// Create an old backup and a recent one.
	oldBackup := path + ".2000-01-01-00-00-00"
	recentBackup := path + ".2999-01-01-00-00-00"
	require.NoError(t, os.WriteFile(oldBackup, []byte("old"), filePermissions))
	require.NoError(t, os.WriteFile(recentBackup, []byte("recent"), filePermissions))
	old := time.Now().Add(-40 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldBackup, old, old))

	w.mu.Lock()
	w.cleanup()
	w.mu.Unlock()

	_, err = os.Stat(oldBackup)
	assert.True(t, os.IsNotExist(err), "backup older than MaxAgeDays should be removed")
	_, err = os.Stat(recentBackup)
	assert.NoError(t, err, "recent backup should be kept")
}

func TestCompressOnRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: 1, Compress: true})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	chunk := make([]byte, 600*1024)
	_, err = w.Write(chunk)
	require.NoError(t, err)
	_, err = w.Write(chunk)
	require.NoError(t, err)

	rotated := backups(t, path)
	require.Len(t, rotated, 1)
	assert.True(t, filepath.Ext(rotated[0]) == ".gz", "rotated file should be gzip-compressed: %s", rotated[0])
}

func TestNextBoundary(t *testing.T) {
	loc := time.Local
	base := time.Date(2026, 7, 6, 14, 30, 0, 0, loc)

	assert.Equal(t, time.Date(2026, 7, 7, 0, 0, 0, 0, loc), nextBoundary(base, 1))
	assert.Equal(t, time.Date(2026, 7, 8, 0, 0, 0, 0, loc), nextBoundary(base, 2))

	// At exactly midnight it schedules the next boundary, not the current instant.
	midnight := time.Date(2026, 7, 6, 0, 0, 0, 0, loc)
	assert.Equal(t, time.Date(2026, 7, 7, 0, 0, 0, 0, loc), nextBoundary(midnight, 1))

	// An interval below 1 is treated as 1.
	assert.Equal(t, time.Date(2026, 7, 7, 0, 0, 0, 0, loc), nextBoundary(base, 0))
}

func TestScheduledRotationRotatesNonEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	_, err = w.Write([]byte("some content\n"))
	require.NoError(t, err)

	w.rotateOnSchedule()
	assert.Len(t, backups(t, path), 1, "scheduled rotation should roll a non-empty file")
}

func TestScheduledRotationSkipsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	w.rotateOnSchedule()
	assert.Empty(t, backups(t, path), "scheduled rotation should skip an empty file")
}

func TestCloseStopsTimeRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, IntervalDays: 1})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- w.Close() }()
	select {
	case cerr := <-done:
		assert.NoError(t, cerr)
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return; the time-rotation goroutine was not stopped")
	}
}

func TestCloseRejectsFurtherWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path})
	require.NoError(t, err)

	require.NoError(t, w.Close())
	assert.NoError(t, w.Close(), "Close should be idempotent")

	_, err = w.Write([]byte("after close"))
	assert.Error(t, err)
}

func TestConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thunderid.log")
	w, err := New(Config{Path: path, MaxSizeMB: 1})
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	var wg sync.WaitGroup
	line := []byte("concurrent-log-line\n")
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, werr := w.Write(line); werr != nil {
					t.Errorf("write failed: %v", werr)
				}
			}
		}()
	}
	wg.Wait()
}
