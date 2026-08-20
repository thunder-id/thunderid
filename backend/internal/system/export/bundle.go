// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteBundle writes every exported file (and the env sidecar, if present) under outDir, preserving
// each file's relative folder path.
func WriteBundle(outDir string, response *ExportResponse) error {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory %q: %w", outDir, err)
	}
	for _, f := range response.Files {
		dir := filepath.Join(outDir, filepath.Clean(f.FolderPath))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("failed to create %q: %w", dir, err)
		}
		path := filepath.Join(dir, f.FileName)
		if err := os.WriteFile(path, []byte(f.Content), 0o600); err != nil {
			return fmt.Errorf("failed to write %q: %w", path, err)
		}
	}
	if response.EnvFile != nil {
		path := filepath.Join(outDir, response.EnvFile.FileName)
		if err := os.WriteFile(path, []byte(response.EnvFile.Content), 0o600); err != nil {
			return fmt.Errorf("failed to write env file %q: %w", path, err)
		}
	}
	return nil
}
