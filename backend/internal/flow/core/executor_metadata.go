// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// ExecutorMetadataProvider is the read-only, runtime-free view of the executor registry that flow
// definition validation and graph building require. It exposes only the static metadata surface,
// a registration check and the executor metadata, and never a live executor instance, so a consumer
// that depends on it does not link the runtime executor constructors or the services they need.
// The full executor registry used by the flow execution engine satisfies this interface.
type ExecutorMetadataProvider interface {
	IsRegistered(name string) bool
	GetExecutorMeta(name string) (*providers.ExecutorMeta, error)
}
