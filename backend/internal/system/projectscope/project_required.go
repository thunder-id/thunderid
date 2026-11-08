// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package projectscope decides, for each resource type, whether a resource must be created in a project,
// and holds the project error codes other services match. It is apart from the project package, which
// only the Control Plane links.
package projectscope

import "github.com/thunder-id/thunderid/internal/system/security"

// projectRequired lists the resource types that must be created in a project on a deployment that serves
// projects. A type set to false may still be created without one while its clients migrate.
var projectRequired = map[security.ResourceType]bool{
	security.ResourceTypeApplication:          false,
	security.ResourceTypeAgent:                false,
	security.ResourceTypeFlow:                 false,
	security.ResourceTypeNotificationTemplate: true,
	security.ResourceTypeOU:                   true,
	security.ResourceTypeVerifiableCredential: true,
}

// ProjectRequired reports whether a resource of the given type must be created in a project.
func ProjectRequired(resourceType security.ResourceType) bool {
	return projectRequired[resourceType]
}
