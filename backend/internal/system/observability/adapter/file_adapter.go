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

// Package adapter provides output adapters for observability events.
package adapter

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	defaultBufferSize    = 4096
	defaultFlushInterval = 5 * time.Second
	defaultMaxFileSizeMB = 100
	defaultMaxBackups    = 10
	defaultMaxAgeDays    = 30
	megabyte             = 1024 * 1024
	loggerComponentName  = "FileOutputAdapter"
	dirPermissions       = 0750
	filePermissions      = 0600
)

// Config holds configuration for the file adapter.
type Config struct {
	// Path is the file path to write events to (required)
	Path string

	// Rotation settings (optional - all default to 0 which disables rotation)
	MaxFileSizeMB int  // Maximum file size in MB before rotation (0 = no rotation)
	MaxBackups    int  // Maximum number of old log files to keep (0 = keep all)
	MaxAgeDays    int  // Maximum age in days for old log files (0 = no age limit)
	Compress      bool // Whether to compress rotated files with gzip
}

// FileAdapter writes events to a file with optional rotation support.
// When rotation is disabled (MaxFileSizeMB = 0), it writes to a single file with buffering.
// When rotation is enabled, it automatically rotates files when they reach the size limit.
type FileAdapter struct {
	config      *Config
	file        *os.File
	writer      *bufio.Writer
	currentSize int64
	mu          sync.Mutex
	flushTicker *time.Ticker
	stopFlush   chan struct{}
	wg          sync.WaitGroup
	closed      bool
}

var _ OutputAdapterInterface = (*FileAdapter)(nil)

// NewFileAdapter creates a new file-based output adapter with simple path.
// This uses default settings with no rotation (backward compatible).
func NewFileAdapter(filePath string) (*FileAdapter, error) {
	return NewFileAdapterWithConfig(&Config{
		Path: filePath,
		// No rotation by default
		MaxFileSizeMB: 0,
		MaxBackups:    0,
		MaxAgeDays:    0,
		Compress:      false,
	})
}

// NewFileAdapterWithConfig creates a new file adapter with custom configuration.
// Use this for production deployments with rotation enabled.
func NewFileAdapterWithConfig(config *Config) (*FileAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Path == "" {
		return nil, fmt.Errorf("file path is required")
	}

	// Validate and apply defaults for rotation settings
	if config.MaxFileSizeMB < 0 {
		config.MaxFileSizeMB = 0
	}
	if config.MaxBackups < 0 {
		config.MaxBackups = 0
	}
	if config.MaxAgeDays < 0 {
		config.MaxAgeDays = 0
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open file in append mode
	// #nosec G304 -- File path is provided by configuration
	file, err := os.OpenFile(config.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", config.Path, err)
	}

	// Get current file size
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	fa := &FileAdapter{
		config:      config,
		file:        file,
		writer:      bufio.NewWriterSize(file, defaultBufferSize),
		currentSize: fileInfo.Size(),
		flushTicker: time.NewTicker(defaultFlushInterval),
		stopFlush:   make(chan struct{}),
		closed:      false,
	}

	// Start periodic flushing
	fa.wg.Add(1)
	go fa.periodicFlush()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	if fa.isRotationEnabled() {
		logger.Info("File adapter initialized with rotation",
			log.String("filePath", config.Path),
			log.Int("maxFileSizeMB", config.MaxFileSizeMB),
			log.Int("maxBackups", config.MaxBackups),
			log.Int("maxAgeDays", config.MaxAgeDays),
			log.Bool("compress", config.Compress))
	} else {
		logger.Info("File adapter initialized",
			log.String("filePath", config.Path))
	}

	return fa, nil
}

// isRotationEnabled returns true if file rotation is configured.
func (fa *FileAdapter) isRotationEnabled() bool {
	return fa.config.MaxFileSizeMB > 0
}

// Write writes data to the file, rotating if necessary.
func (fa *FileAdapter) Write(data []byte) error {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	if fa.closed {
		return fmt.Errorf("file adapter is closed")
	}

	// Check if rotation is needed (only if rotation is enabled)
	if fa.isRotationEnabled() {
		dataSize := int64(len(data)) + 1 // +1 for newline
		maxSize := int64(fa.config.MaxFileSizeMB) * megabyte
		if fa.currentSize+dataSize > maxSize {
			if err := fa.rotate(); err != nil {
				logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
				logger.Error("Failed to rotate file", log.Error(err))
				// Continue writing to current file even if rotation fails
			}
		}
	}

	// Write data
	n, err := fa.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	fa.currentSize += int64(n)

	// Write newline
	n, err = fa.writer.WriteString("\n")
	if err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}
	fa.currentSize += int64(n)

	return nil
}

// rotate rotates the log file (must be called with lock held).
func (fa *FileAdapter) rotate() error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// Flush and close current writer and file
	if err := fa.writer.Flush(); err != nil {
		logger.Error("Failed to flush before rotation", log.Error(err))
	}
	if err := fa.file.Close(); err != nil {
		logger.Error("Failed to close file during rotation", log.Error(err))
	}

	// Generate rotated filename with timestamp
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	rotatedPath := fa.config.Path + "." + timestamp

	// Rename current file
	if err := os.Rename(fa.config.Path, rotatedPath); err != nil {
		logger.Error("Failed to rename file during rotation", log.Error(err))
		// Try to reopen original file
		file, err := os.OpenFile(fa.config.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermissions)
		if err != nil {
			return fmt.Errorf("failed to reopen file after rotation error: %w", err)
		}
		fa.file = file
		fa.writer = bufio.NewWriterSize(file, defaultBufferSize)
		fa.currentSize = 0
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Compress rotated file if enabled
	if fa.config.Compress {
		go fa.compressFile(rotatedPath)
	}

	// Create new file
	file, err := os.OpenFile(fa.config.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermissions)
	if err != nil {
		return fmt.Errorf("failed to create new file after rotation: %w", err)
	}

	fa.file = file
	fa.writer = bufio.NewWriterSize(file, defaultBufferSize)
	fa.currentSize = 0

	logger.Info("File rotated successfully", log.String("rotatedFile", rotatedPath))

	// Clean up old files
	go fa.cleanup()

	return nil
}

// compressFile compresses a log file using gzip.
func (fa *FileAdapter) compressFile(filePath string) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// Open source file
	src, err := os.Open(filePath) // #nosec G304 -- File path is controlled internally by rotation logic
	if err != nil {
		logger.Error("Failed to open file for compression", log.String("filePath", filePath), log.Error(err))
		return
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			logger.Error("Failed to close source file", log.Error(closeErr))
		}
	}()

	// Create compressed file
	compressedPath := filePath + ".gz"
	dst, err := os.Create(compressedPath) // #nosec G304 -- File path is derived from controlled internal path
	if err != nil {
		logger.Error("Failed to create compressed file", log.String("compressedPath", compressedPath), log.Error(err))
		return
	}
	defer func() {
		if closeErr := dst.Close(); closeErr != nil {
			logger.Error("Failed to close destination file", log.Error(closeErr))
		}
	}()

	// Create gzip writer
	gzWriter := gzip.NewWriter(dst)
	defer func() {
		if closeErr := gzWriter.Close(); closeErr != nil {
			logger.Error("Failed to close gzip writer", log.Error(closeErr))
		}
	}()

	// Copy and compress
	if _, err := io.Copy(gzWriter, src); err != nil {
		logger.Error("Failed to compress file", log.String("filePath", filePath), log.Error(err))
		return
	}

	// Close gzip writer to flush
	if err := gzWriter.Close(); err != nil {
		logger.Error("Failed to close gzip writer", log.Error(err))
		return
	}

	// Remove original file
	if err := os.Remove(filePath); err != nil {
		logger.Error("Failed to remove original file after compression",
			log.String("filePath", filePath), log.Error(err))
		return
	}

	logger.Info("File compressed successfully", log.String("compressedPath", compressedPath))
}

// cleanup removes old log files based on maxBackups and maxAge.
func (fa *FileAdapter) cleanup() {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	dir := filepath.Dir(fa.config.Path)
	baseName := filepath.Base(fa.config.Path)

	// Find all rotated log files
	files, err := filepath.Glob(filepath.Join(dir, baseName+".*"))
	if err != nil {
		logger.Error("Failed to list rotated files", log.Error(err))
		return
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		fi, err1 := os.Stat(files[i])
		fj, err2 := os.Stat(files[j])
		if err1 != nil || err2 != nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})

	now := time.Now()
	removed := 0

	for i, file := range files {
		// Skip if it's the current file
		if file == fa.config.Path {
			continue
		}

		shouldRemove := false

		// Check maxBackups
		if fa.config.MaxBackups > 0 && i >= fa.config.MaxBackups {
			shouldRemove = true
		}

		// Check maxAge
		if fa.config.MaxAgeDays > 0 {
			fileInfo, err := os.Stat(file)
			if err == nil {
				age := now.Sub(fileInfo.ModTime())
				if age > time.Duration(fa.config.MaxAgeDays)*24*time.Hour {
					shouldRemove = true
				}
			}
		}

		if shouldRemove {
			if err := os.Remove(file); err != nil {
				logger.Error("Failed to remove old log file", log.String("filePath", file), log.Error(err))
			} else {
				removed++
			}
		}
	}

	if removed > 0 {
		logger.Info("Cleaned up old log files", log.Int("removedCount", removed))
	}
}

// Flush flushes buffered data to the file.
func (fa *FileAdapter) Flush() error {
	fa.mu.Lock()
	defer fa.mu.Unlock()

	if fa.closed {
		return nil
	}

	if err := fa.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	if err := fa.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}

// periodicFlush periodically flushes the buffer to ensure data is persisted.
func (fa *FileAdapter) periodicFlush() {
	defer fa.wg.Done()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	for {
		select {
		case <-fa.flushTicker.C:
			if err := fa.Flush(); err != nil {
				logger.Error("Failed to flush file", log.Error(err))
			}
		case <-fa.stopFlush:
			return
		}
	}
}

// Close closes the file adapter and releases resources.
func (fa *FileAdapter) Close() error {
	fa.mu.Lock()
	if fa.closed {
		fa.mu.Unlock()
		return nil
	}
	fa.closed = true
	fa.mu.Unlock()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Info("Closing file adapter", log.String("filePath", fa.config.Path))

	// Stop periodic flushing
	fa.flushTicker.Stop()
	close(fa.stopFlush)
	fa.wg.Wait()

	// Final flush
	if err := fa.Flush(); err != nil {
		logger.Error("Failed to perform final flush", log.Error(err))
	}

	// Close file
	if err := fa.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	logger.Info("File adapter closed")
	return nil
}

// GetName returns the name of this adapter.
func (fa *FileAdapter) GetName() string {
	return "FileAdapter"
}
