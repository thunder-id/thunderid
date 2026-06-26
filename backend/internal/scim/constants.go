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

// Package scim implements the SCIM v2.0 API endpoints for ThunderID,
// following RFC 7643 and RFC 7644.
package scim

const (
	loggerComponentName              = "SCIMhandler"
	scimServiceProviderConfigCreated = "2025-01-01T00:00:00Z"

	// SCIMBasePath is the base path for all SCIM v2 endpoints.
	SCIMBasePath = "/scim/v2"

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

	// scimCoreUserSchemaCreated is the static creation timestamp for the core User schema.
	// This resource is static and never mutated by operators.
	scimCoreUserSchemaCreated = "2025-01-01T00:00:00Z"

	scimAttrTypeString  = "string"
	scimAttrTypeInteger = "integer"
	scimAttrTypeDecimal = "decimal"
	scimAttrTypeBoolean = "boolean"
	scimAttrTypeComplex = "complex"

	scimMutabilityReadWrite = "readWrite"
	scimMutabilityReadOnly  = "readOnly"
	scimMutabilityImmutable = "immutable"
	scimMutabilityWriteOnly = "writeOnly"

	scimReturnedAlways  = "always"
	scimReturnedNever   = "never"
	scimReturnedDefault = "default"

	scimUniquenessNone   = "none"
	scimUniquenessServer = "server"
	scimUniquenessGlobal = "global"

	scimResourceTypeUserID       = "User"
	scimResourceTypeUserName     = "User"
	scimResourceTypeUserEndpoint = "/Users"
	scimResourceTypeUserDesc     = "User Account"
)
