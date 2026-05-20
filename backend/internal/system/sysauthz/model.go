/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package sysauthz

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// OUHierarchyResolver provides read-only traversal of the organization unit tree.
// It is defined here in the sysauthz package (rather than in the ou package) to break the
// potential import cycle: ou already imports sysauthz for its authz checks, so if sysauthz
// were to import ou for hierarchy traversal it would form a cycle.
// The ou package implements this interface and injects a concrete instance via
// SystemAuthorizationServiceInterface.SetOUHierarchyResolver at application startup.
type OUHierarchyResolver interface {
	// IsAncestor returns true when ancestorOUID appears anywhere in the parent
	// chain above descendantOUID.
	// A non-nil ServiceError indicates a traversal failure; the caller should treat the
	// result as false (deny-safe).
	IsAncestor(ctx context.Context, ancestorOUID, descendantOUID string) (bool, *serviceerror.ServiceError)

	// GetAncestorOUIDs returns every ancestor OU ID walking up
	// to the root of the tree. A non-nil ServiceError indicates a traversal failure.
	GetAncestorOUIDs(ctx context.Context, ouID string) ([]string, *serviceerror.ServiceError)
}

// ActionContext provides contextual information used to make an authorization decision.
// Not all fields are required for every action; populate only those relevant to the operation.
type ActionContext struct {
	// OUID is the organization unit ID scoping the action.
	// Leave empty if the action is not scoped to a specific OU.
	OUID string
	// ResourceType is the type of resource being acted upon.
	ResourceType security.ResourceType
	// ResourceID is the identifier of the specific resource being acted upon.
	// Leave empty for collection-level actions (e.g., list, create).
	ResourceID string
}

// AccessibleResources represents the set of resources a caller is permitted to access
// for a given action. It is used to pre-filter store queries before pagination is applied.
type AccessibleResources struct {
	// AllAllowed signals that the caller may access all resources of the requested type.
	// When true, the caller should apply no ID filter to the store query.
	// When false, only the IDs listed in IDs are accessible.
	AllAllowed bool
	// IDs is the explicit set of accessible resource IDs.
	// Only populated when AllAllowed is false.
	IDs []string
}
