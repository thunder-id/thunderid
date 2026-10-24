// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
)

// Initialize creates an executor registry and registers built-in executors.
// When flowConfig.Flow.Executors is empty, all built-in executors are registered.
// When non-empty, only the listed executors are registered; flows using other executors
// will fail validation until those executors are included or the list is cleared.
func Initialize(deps ExecutorDependencies, flowConfig flowconfig.Config) (ExecutorRegistryInterface, error) {
	reg := newExecutorRegistry()
	deps.tokenLeeway = flowConfig.TokenLeeway
	names := flowConfig.Flow.Executors
	if err := registerBuiltInExecutors(reg, deps, names); err != nil {
		return nil, err
	}
	return reg, nil
}
