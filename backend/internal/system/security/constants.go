// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

const (
	// maxPublicPathLength defines the maximum allowed length for a public path.
	// This prevents potential DoS attacks via excessively long paths (even with safe regex).
	maxPublicPathLength = 4096

	// maxAPIPermissionPathLength defines the maximum request path length evaluated against the API
	// permission map. The map holds one anchored pattern per entry and matching costs time linear in
	// the path length, so a longer path is treated as matching no entry rather than being scanned.
	maxAPIPermissionPathLength = 4096
)
