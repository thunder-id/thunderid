// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

// scimAttrType represents SCIM attribute data types (RFC 7643 §2.3).
type scimAttrType string

const (
	scimAttrTypeString  scimAttrType = "string"
	scimAttrTypeInteger scimAttrType = "integer"
	scimAttrTypeDecimal scimAttrType = "decimal"
	scimAttrTypeBoolean scimAttrType = "boolean"
	scimAttrTypeComplex scimAttrType = "complex"
)

// scimMutability represents SCIM attribute mutability values (RFC 7643 §7).
type scimMutability string

const (
	scimMutabilityReadWrite scimMutability = "readWrite"
	scimMutabilityReadOnly  scimMutability = "readOnly"
	scimMutabilityImmutable scimMutability = "immutable"
	scimMutabilityWriteOnly scimMutability = "writeOnly"
)

// scimReturned represents SCIM attribute returned values (RFC 7643 §7).
type scimReturned string

const (
	scimReturnedAlways  scimReturned = "always"
	scimReturnedNever   scimReturned = "never"
	scimReturnedDefault scimReturned = "default"
)

// scimUniqueness represents SCIM attribute uniqueness values (RFC 7643 §7).
type scimUniqueness string

const (
	scimUniquenessNone   scimUniqueness = "none"
	scimUniquenessServer scimUniqueness = "server"
	scimUniquenessGlobal scimUniqueness = "global"
)

// Resource type metadata.
const (
	scimResourceTypeUserID       = "User"
	scimResourceTypeUserName     = "User"
	scimResourceTypeUserEndpoint = "/Users"
	scimResourceTypeUserDesc     = "User Account"

	scimResourceTypeGroupID       = "Group"
	scimResourceTypeGroupName     = "Group"
	scimResourceTypeGroupEndpoint = "/Groups"
	scimResourceTypeGroupDesc     = "Group"
)

// Discovery-only schema URNs (RFC 7643 §5, RFC 7644 §4). These describe the
// discovery meta-endpoints themselves, not resource payloads, so they are not
// shared with the users/groups packages.
const (
	scimServiceProviderConfigSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	scimResourceTypeSchemaURN          = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	scimSchemaSchemaURN                = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)
