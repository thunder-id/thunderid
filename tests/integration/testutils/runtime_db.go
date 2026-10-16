// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateEntityRuntimeOnlyDB writes a SQLite database at dbPath, relative to the product home, holding
// only the shipped runtime-persistent schema's ENTITY_RUNTIME_DATA table. A server pointed at it reads
// entity runtime rows, while every other runtime-persistent query fails. Needs the sqlite3 CLI.
func CreateEntityRuntimeOnlyDB(dbPath string) error {
	home := GetExtractedProductHome()
	schema, err := os.ReadFile(filepath.Join(home, "dbscripts", "runtime_persistent", "sqlite.sql"))
	if err != nil {
		return fmt.Errorf("failed to read the runtime-persistent schema: %w", err)
	}
	start := strings.Index(string(schema), `CREATE TABLE "ENTITY_RUNTIME_DATA"`)
	if start < 0 {
		return fmt.Errorf("the runtime-persistent schema has no ENTITY_RUNTIME_DATA table")
	}
	end := strings.Index(string(schema[start:]), ");")
	if end <= 0 {
		return fmt.Errorf("unterminated ENTITY_RUNTIME_DATA definition")
	}

	fullPath := filepath.Join(home, dbPath)
	_ = os.Remove(fullPath)
	cmd := exec.Command("sqlite3", fullPath)
	cmd.Stdin = strings.NewReader(string(schema[start:start+end+2]) + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create %s: %v: %s", dbPath, err, strings.TrimSpace(string(out)))
	}
	return nil
}
