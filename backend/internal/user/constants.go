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
