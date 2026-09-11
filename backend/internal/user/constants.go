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

// CredentialTypePassword is the schema-defined password credential. It is not system-managed, but
// it is named here because it acts as the account-level proof of ownership when a user changes
// their own credentials.
const CredentialTypePassword CredentialType = "password"

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
