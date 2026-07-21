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

// Package rollingfile provides an io.Writer that writes to a file and rotates
// it once it reaches a configured size, keeping a bounded number of gzip-able
// backups. It is used as the destination writer behind the application logger's
// slog handler, so it deliberately does no formatting of its own.
package rollingfile

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DefaultMaxSizeMB is the fallback rotation size (in MB) callers can use when
// size rotation is enabled but no explicit size is configured.
const DefaultMaxSizeMB float64 = 10

// DefaultIntervalDays is the fallback rotation interval callers can use when
// time rotation is enabled but no explicit interval is configured.
const DefaultIntervalDays = 1

const (
	megabyte           = 1024 * 1024
	dirPermissions     = 0750
	filePermissions    = 0600
	rotationTimeFormat = "2006-01-02-15-04-05"
)

// Config holds the settings for a rotating file writer.
type Config struct {
	// Path is the full path of the active log file (required).
	Path string
	// MaxSizeMB is the size (in MB, fractional allowed) at which the file is
	// rotated. 0 disables size rotation.
	MaxSizeMB float64
	// IntervalDays rotates the file every N days at the local calendar boundary
	// (midnight). 0 disables time-based rotation.
	IntervalDays int
	// MaxBackups is the maximum number of rotated files to retain. 0 keeps all.
	MaxBackups int
	// MaxAgeDays is the maximum age of a rotated file before deletion. 0 disables age-based deletion.
	MaxAgeDays int
	// Compress controls whether rotated files are gzip-compressed.
	Compress bool
}

// Writer is a size- and/or time-rotating file writer that implements io.Writer.
type Writer struct {
	config      Config
	mu          sync.Mutex
	file        *os.File
	currentSize int64
	closed      bool

	// stop signals the time-rotation goroutine to exit; nil when time rotation
	// is disabled.
	stop chan struct{}
	wg   sync.WaitGroup
}

var _ io.WriteCloser = (*Writer)(nil)

// New opens (creating if needed) the file at cfg.Path and returns a Writer. When
// time-based rotation is configured, it starts a background goroutine that rotates
// the file at each calendar boundary.
func New(cfg Config) (*Writer, error) {
	if cfg.Path == "" {
		return nil, errors.New("rollingfile: path is required")
	}
	if cfg.MaxSizeMB < 0 {
		cfg.MaxSizeMB = 0
	}
	if cfg.IntervalDays < 0 {
		cfg.IntervalDays = 0
	}
	if cfg.MaxBackups < 0 {
		cfg.MaxBackups = 0
	}
	if cfg.MaxAgeDays < 0 {
		cfg.MaxAgeDays = 0
	}

	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return nil, fmt.Errorf("rollingfile: failed to create directory %s: %w", dir, err)
	}

	file, size, err := openAppend(cfg.Path)
	if err != nil {
		return nil, err
	}

	w := &Writer{config: cfg, file: file, currentSize: size}
	if cfg.IntervalDays > 0 {
		w.stop = make(chan struct{})
		w.wg.Add(1)
		go w.runTimeRotation()
	}
	return w, nil
}

// openAppend opens the file for appending and returns it with its current size.
func openAppend(path string) (*os.File, int64, error) {
	cleanPath := filepath.Clean(path)
	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermissions)
	if err != nil {
		return nil, 0, fmt.Errorf("rollingfile: failed to open file %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("rollingfile: failed to stat file %s: %w", path, err)
	}
	return file, info.Size(), nil
}

// rotationEnabled reports whether size-based rotation is configured.
func (w *Writer) rotationEnabled() bool {
	return w.config.MaxSizeMB > 0
}

// runTimeRotation rotates the file at each calendar boundary until stopped. It
// aligns to local midnight (so a daily interval rolls at 00:00, not 24h after
// startup), matching the timezone slog prints its timestamps in.
func (w *Writer) runTimeRotation() {
	defer w.wg.Done()
	for {
		now := time.Now()
		timer := time.NewTimer(nextBoundary(now, w.config.IntervalDays).Sub(now))
		select {
		case <-timer.C:
			w.rotateOnSchedule()
		case <-w.stop:
			timer.Stop()
			return
		}
	}
}

// rotateOnSchedule performs a time-triggered rotation, skipping an empty file so
// idle days don't produce empty backups.
func (w *Writer) rotateOnSchedule() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.currentSize == 0 {
		return
	}
	if err := w.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "rollingfile: scheduled rotation failed: %v\n", err)
	}
}

// nextBoundary returns the next local calendar rotation instant: midnight,
// intervalDays after the start of the current day.
func nextBoundary(now time.Time, intervalDays int) time.Time {
	if intervalDays < 1 {
		intervalDays = 1
	}
	year, month, day := now.Date()
	startOfDay := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	return startOfDay.AddDate(0, 0, intervalDays)
}

// Write appends p to the active file, rotating first if the write would exceed
// the configured size. It never splits p across files.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, errors.New("rollingfile: writer is closed")
	}

	// Recover from a prior rotation failure that left the writer without an open
	// file, so a transient OS error doesn't silence file logging permanently.
	if w.file == nil {
		file, size, err := openAppend(w.config.Path)
		if err != nil {
			return 0, err
		}
		w.file = file
		w.currentSize = size
	}

	if w.rotationEnabled() {
		maxSize := int64(w.config.MaxSizeMB * megabyte)
		// Only rotate a non-empty file, so a single record larger than the limit
		// does not rotate an empty file on every write.
		if w.currentSize > 0 && w.currentSize+int64(len(p)) > maxSize {
			if err := w.rotate(); err != nil {
				return 0, err
			}
		}
	}

	n, err := w.file.Write(p)
	w.currentSize += int64(n)
	return n, err
}

// rotate closes the active file, renames it with a timestamp suffix, optionally
// compresses it, opens a fresh active file, and prunes old backups. It must be
// called with the mutex held.
func (w *Writer) rotate() error {
	if err := w.file.Close(); err != nil {
		// Closing can fail without leaving a usable descriptor; log and continue so
		// a transient close error does not permanently block rotation. The file is
		// reopened (or w.file cleared) below in every path.
		fmt.Fprintf(os.Stderr, "rollingfile: failed to close file during rotation: %v\n", err)
	}

	rotatedPath := w.uniqueRotatedPath(time.Now())
	if err := os.Rename(w.config.Path, rotatedPath); err != nil {
		// Reopen the original file so logging can continue despite the failure.
		file, size, reopenErr := openAppend(w.config.Path)
		if reopenErr != nil {
			// Leave the writer without a file so the next Write retries opening it
			// instead of writing to a closed descriptor forever.
			w.file = nil
			return fmt.Errorf("rollingfile: rename failed and reopen failed: %w", reopenErr)
		}
		w.file = file
		w.currentSize = size
		return fmt.Errorf("rollingfile: failed to rename file during rotation: %w", err)
	}

	if w.config.Compress {
		if err := compressFile(rotatedPath); err != nil {
			// Compression is best-effort; keep the uncompressed backup and carry on.
			fmt.Fprintf(os.Stderr, "rollingfile: failed to compress %s: %v\n", rotatedPath, err)
		}
	}

	file, _, err := openAppend(w.config.Path)
	if err != nil {
		// Leave the writer without a file so the next Write retries opening it.
		w.file = nil
		return err
	}
	w.file = file
	w.currentSize = 0

	w.cleanup()
	return nil
}

// uniqueRotatedPath returns a rotated file path that does not collide with an
// existing backup (plain or compressed), appending a counter when multiple
// rotations land within the one-second timestamp resolution.
func (w *Writer) uniqueRotatedPath(now time.Time) string {
	base := w.config.Path + "." + now.Format(rotationTimeFormat)
	candidate := base
	for i := 1; taken(candidate); i++ {
		candidate = fmt.Sprintf("%s.%d", base, i)
	}
	return candidate
}

// taken reports whether a rotated path is already in use in either its plain or
// gzip-compressed form.
func taken(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	if _, err := os.Stat(path + ".gz"); err == nil {
		return true
	}
	return false
}

// cleanup removes rotated files that exceed MaxBackups or MaxAgeDays. It must be
// called with the mutex held.
func (w *Writer) cleanup() {
	if w.config.MaxBackups == 0 && w.config.MaxAgeDays == 0 {
		return
	}

	dir := filepath.Dir(w.config.Path)
	base := filepath.Base(w.config.Path)
	matches, err := filepath.Glob(filepath.Join(dir, base+".*"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "rollingfile: failed to list rotated files: %v\n", err)
		return
	}

	// Newest first, so index-based backup counting keeps the most recent files.
	sort.Slice(matches, func(i, j int) bool {
		fi, err1 := os.Stat(matches[i])
		fj, err2 := os.Stat(matches[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})

	now := time.Now()
	for i, path := range matches {
		if path == w.config.Path {
			continue
		}
		remove := false
		if w.config.MaxBackups > 0 && i >= w.config.MaxBackups {
			remove = true
		}
		if w.config.MaxAgeDays > 0 {
			if info, statErr := os.Stat(path); statErr == nil {
				if now.Sub(info.ModTime()) > time.Duration(w.config.MaxAgeDays)*24*time.Hour {
					remove = true
				}
			}
		}
		if remove {
			if err := os.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "rollingfile: failed to remove old log file %s: %v\n", path, err)
			}
		}
	}
}

// Close stops time-based rotation and closes the active file.
func (w *Writer) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	// Stop the time-rotation goroutine outside the lock so it can never be blocked
	// on the mutex while we wait for it to exit.
	if w.stop != nil {
		close(w.stop)
		w.wg.Wait()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// compressFile gzips the file at path and removes the original on success.
func compressFile(path string) (err error) {
	cleanPath := filepath.Clean(path)
	src, err := os.Open(cleanPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	compressedPath := filepath.Clean(cleanPath + ".gz")
	dst, err := os.OpenFile(compressedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePermissions)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	gz := gzip.NewWriter(dst)
	if _, cerr := io.Copy(gz, src); cerr != nil {
		_ = gz.Close()
		return cerr
	}
	if cerr := gz.Close(); cerr != nil {
		return cerr
	}
	return os.Remove(path)
}
