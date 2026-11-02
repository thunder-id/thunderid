// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// accountAccessConfigSaved is set once the pack's own declarative accountAccess document, or its
// absence, has been recorded.
var (
	accountAccessConfigSaved   bool
	shippedAccountAccessConfig []byte
)

// accountAccessConfigPath is the declarative accountAccess document in the extracted pack.
func accountAccessConfigPath() string {
	return filepath.Join(extractedProductHome, "config", "resources", "server_configs", "account_access.yaml")
}

// UpdateAccountAccessConfig writes a declarative accountAccess document for a test suite. It is laid
// over the deployment defaults and locks the section against writes.
func UpdateAccountAccessConfig(value any) error {
	ensureInitialized()
	path := accountAccessConfigPath()
	if !accountAccessConfigSaved {
		original, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read declarative accountAccess config: %w", err)
		}
		shippedAccountAccessConfig = original
		accountAccessConfigSaved = true
	}
	data, err := yaml.Marshal(map[string]any{
		"resource_type": "server_config", "name": "accountAccess", "value": value,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// RestoreAccountAccessConfig restores the pack's declarative document, removing the one a suite wrote
// when the pack had none.
func RestoreAccountAccessConfig() error {
	if !accountAccessConfigSaved {
		return nil
	}
	path := accountAccessConfigPath()
	if shippedAccountAccessConfig == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, shippedAccountAccessConfig, 0600)
}
