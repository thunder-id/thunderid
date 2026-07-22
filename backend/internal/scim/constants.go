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

package scim

// SCIMBasePath is the base path for all SCIM v2 endpoints.
const SCIMBasePath = "/scim/v2"

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

	// ThunderIDURNSuffix is the custom URN suffix for ThunderID SCIM user schemas.
	ThunderIDURNSuffix = ":2.0:User"

	// SCIMCoreGroupSchemaURN is the SCIM core Group schema URN.
	SCIMCoreGroupSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:Group"

	// SCIMPatchOpSchemaURN is the SCIM PatchOp schema URN.
	SCIMPatchOpSchemaURN = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
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

// Raw entity-type schema property type strings, compared case-insensitively
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
	scimErrorTypeInvalidValue = "invalidValue"
	scimErrorTypeInvalidPath  = "invalidPath"
)
