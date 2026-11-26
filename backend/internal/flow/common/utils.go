// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

// ExtensionNamespace returns the prefix of the runtime-data keys the extension step nodeID stores.
func ExtensionNamespace(nodeID string) string {
	return ExtensionDataKeyPrefix + nodeID + "."
}
