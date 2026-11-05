// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

// The system flow a role update runs before it drops a permission, and the input carrying the role's new
// permission set.
const (
	systemFlowRolePermissionRemovalPre = "system-role-permission-removal-pre"
	systemFlowInputPermissions         = "targetPermissions"
)
