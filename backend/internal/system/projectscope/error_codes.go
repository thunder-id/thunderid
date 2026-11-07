// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package projectscope

// Error codes the project service returns when it checks the organization unit of a resource in a project.
// Services on either plane match them here, without linking the Control Plane only project package.
const (
	// ProjectNotFoundErrorCode is returned when no project exists with the given ID.
	ProjectNotFoundErrorCode = "PRJ-1002"
	// OUNotInProjectErrorCode is returned when an organization unit is outside the project.
	OUNotInProjectErrorCode = "PRJ-1010"
)
