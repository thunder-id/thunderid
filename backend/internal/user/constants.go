// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import "slices"

// CredentialType represents the type of credential.
type CredentialType string

// Credential type constants for system-managed credential types.
// System-managed credentials are not defined in user types.
const (
	CredentialTypePasskey CredentialType = "passkey"
)

// systemManagedCredentialTypes defines credential types that are managed by the system,
// not through user types. These may support multiple values per user.
var systemManagedCredentialTypes = []CredentialType{
	CredentialTypePasskey,
}

// String returns the string representation of the credential type.
func (ct CredentialType) String() string {
	return string(ct)
}

// IsSystemManaged checks if the credential type is a system-managed credential type.
func (ct CredentialType) IsSystemManaged() bool {
	return slices.Contains(systemManagedCredentialTypes, ct)
}

// sharingPoliciesSegment is the path segment the sharing policy routes hang off.
const sharingPoliciesSegment = "sharing-policies"

// queryParamOUID is the query parameter that names the organization unit a request is about.
const queryParamOUID = "ouId"

// queryParamProjectID is the query parameter that lists the users of one project.
const queryParamProjectID = "projectId"

// userDetail is a section get_user adds to a user only when asked.
type userDetail string

// The sections get_user adds when asked.
const (
	userDetailGroups      userDetail = "groups"
	userDetailRoles       userDetail = "roles"
	userDetailPermissions userDetail = "permissions"
)

// userDetails lists the sections get_user can add, in the order it reports them.
var userDetails = []userDetail{userDetailGroups, userDetailRoles, userDetailPermissions}

// User MCP tool defaults.
const (
	// defaultUserPageSize is the number of users a list returns when no limit is given.
	defaultUserPageSize = 30
	// defaultOUHandle is the handle of the organization unit a user is created in when none is named.
	defaultOUHandle = "default"
)
