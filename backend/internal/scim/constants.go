// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

// SCIMBasePath is the base path for all SCIM v2 endpoints.
const SCIMBasePath = "/scim/v2"

// maxRequestBodyBytes caps incoming SCIM request bodies to 1 MiB.
const maxRequestBodyBytes = 1 << 20

// Schema URNs.
const (
	// SCIMCoreUserSchemaURN is the SCIM core User schema URN.
	SCIMCoreUserSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:User"

	// SCIMErrorSchemaURN is the SCIM error schema URN.
	SCIMErrorSchemaURN = "urn:ietf:params:scim:api:messages:2.0:Error"

	// SCIMListResponseSchemaURN is the SCIM list response schema URN.
	SCIMListResponseSchemaURN = "urn:ietf:params:scim:api:messages:2.0:ListResponse"

	// SCIMServiceProviderConfigSchemaURN is the SCIM ServiceProviderConfig schema URN.
	SCIMServiceProviderConfigSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"

	// SCIMResourceTypeSchemaURN is the SCIM ResourceType schema URN.
	SCIMResourceTypeSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"

	// SCIMSchemaSchemaURN is the SCIM Schema schema URN.
	SCIMSchemaSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:Schema"

	// ThunderIDURNPrefix is the custom URN prefix for ThunderID SCIM schemas.
	ThunderIDURNPrefix = "urn:thunderid:params:scim:schemas:"

	// ThunderIDURNVersion is the custom URN version for ThunderID SCIM schemas.
	ThunderIDURNVersion = ":2.0:"

	// ThunderIDUserURNResource is the custom URN resource name for ThunderID SCIM user schemas.
	ThunderIDUserURNResource = "User"

	// ThunderIDURNSuffix is the custom URN suffix for ThunderID SCIM user schemas.
	ThunderIDURNSuffix = ThunderIDURNVersion + ThunderIDUserURNResource

	// SCIMCoreGroupSchemaURN is the SCIM core Group schema URN.
	SCIMCoreGroupSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:Group"

	// SCIMPatchOpSchemaURN is the SCIM PatchOp schema URN.
	SCIMPatchOpSchemaURN = "urn:ietf:params:scim:api:messages:2.0:PatchOp"

	// SCIMSearchSchemaURN is the SCIM .search schema URN
	SCIMSearchSchemaURN = "urn:ietf:params:scim:api:messages:2.0:SearchRequest"
)

// SCIMAttrType represents SCIM attribute data types (RFC 7643 §2.3).
type SCIMAttrType string

const (
	scimAttrTypeString  SCIMAttrType = "string"
	scimAttrTypeInteger SCIMAttrType = "integer"
	scimAttrTypeDecimal SCIMAttrType = "decimal"
	scimAttrTypeBoolean SCIMAttrType = "boolean"
	scimAttrTypeComplex SCIMAttrType = "complex"
)

// SCIMMutability represents SCIM attribute mutability values (RFC 7643 §7).
type SCIMMutability string

const (
	scimMutabilityReadWrite SCIMMutability = "readWrite"
	scimMutabilityReadOnly  SCIMMutability = "readOnly"
	scimMutabilityImmutable SCIMMutability = "immutable"
	scimMutabilityWriteOnly SCIMMutability = "writeOnly"
)

// SCIMReturned represents SCIM attribute returned values (RFC 7643 §7).
type SCIMReturned string

const (
	scimReturnedAlways  SCIMReturned = "always"
	scimReturnedNever   SCIMReturned = "never"
	scimReturnedDefault SCIMReturned = "default"
)

// SCIMUniqueness represents SCIM attribute uniqueness values (RFC 7643 §7).
type SCIMUniqueness string

const (
	scimUniquenessNone   SCIMUniqueness = "none"
	scimUniquenessServer SCIMUniqueness = "server"
	scimUniquenessGlobal SCIMUniqueness = "global"
)

// Raw user-type schema property type strings, compared case-insensitively
// against rawPropertyDef.Type / rawPropertyDef.Items.Type.
const (
	rawPropertyTypeArray  = "array"
	rawPropertyTypeObject = "object"
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

// SCIM Error types
const (
	scimErrorTypeInvalidValue  = "invalidValue"
	scimErrorTypeInvalidPath   = "invalidPath"
	scimErrorTypeMutability    = "mutability"
	scimErrorTypeInvalidFilter = "invalidFilter"
)

// SCIM PATCH operation values (RFC 7644 §3.5.2).
const (
	scimPatchOpAdd     = "add"
	scimPatchOpRemove  = "remove"
	scimPatchOpReplace = "replace"
)
