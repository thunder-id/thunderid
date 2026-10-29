// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import "github.com/thunder-id/thunderid/internal/sharing"

// NewRoleExporterForTest creates a new role exporter for testing purposes.
func NewRoleExporterForTest(
	service RoleServiceInterface, assignmentService RoleAssignmentServiceInterface,
	sharingService sharing.SharingServiceInterface,
) *roleExporter {
	return newRoleExporter(service, assignmentService, sharingService)
}
