// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

// SCIMBasePath is the base path for all SCIM v2 endpoints.
const SCIMBasePath = "/scim/v2"

// MaxRequestBodyBytes caps incoming SCIM request bodies to 1 MiB.
const MaxRequestBodyBytes = 1 << 20

// Schema URNs.
const (
	// SCIMCoreUserSchemaURN is the SCIM core User schema URN.
	SCIMCoreUserSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:User"

	// SCIMErrorSchemaURN is the SCIM error schema URN.
	SCIMErrorSchemaURN = "urn:ietf:params:scim:api:messages:2.0:Error"

	// SCIMListResponseSchemaURN is the SCIM list response schema URN.
	SCIMListResponseSchemaURN = "urn:ietf:params:scim:api:messages:2.0:ListResponse"

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

// Raw user-type schema property type strings, compared case-insensitively
// against RawPropertyDef.Type / RawPropertyDef.Items.Type. Used by both the
// users (attribute mapping) and discovery (schema building) packages.
const (
	RawPropertyTypeString  = "string"
	RawPropertyTypeNumber  = "number"
	RawPropertyTypeBoolean = "boolean"
	RawPropertyTypeArray   = "array"
	RawPropertyTypeObject  = "object"
)

// ScimErrorType represents SCIM error response scimType values (RFC 7644 §3.12).
type ScimErrorType string

// SCIM Error types. Exported because tests in the users/groups/discovery
// packages assert on the ScimType field of a returned SCIMErrorResponse.
const (
	ScimErrorTypeInvalidValue   ScimErrorType = "invalidValue"
	ScimErrorTypeInvalidPath    ScimErrorType = "invalidPath"
	ScimErrorTypeMutability     ScimErrorType = "mutability"
	ScimErrorTypeInvalidFilter  ScimErrorType = "invalidFilter"
	ScimErrorTypeInvalidSyntax  ScimErrorType = "invalidSyntax"
	ScimErrorTypeUniqueness     ScimErrorType = "uniqueness"
	ScimErrorTypeNotImplemented ScimErrorType = "notImplemented"
)
