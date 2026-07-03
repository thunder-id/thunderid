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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package scimconfig provides the SCIM service configuration.
package scimconfig

import "github.com/thunder-id/thunderid/internal/system/config"

// Static SCIM protocol capability declarations.
// These values reflect what this server implementation supports and are
// not operator-configurable — they are facts about the codebase, not
// deployment decisions.
const (
	// PatchSupported indicates that the SCIM PATCH operation is supported
	// per RFC 7644 §3.5.2.
	PatchSupported = false

	// BulkSupported indicates that SCIM Bulk operations are not yet
	// implemented per RFC 7644 §3.7.
	BulkSupported = false

	// BulkMaxOperations is the maximum number of operations in a Bulk request.
	// Zero because Bulk is not supported.
	BulkMaxOperations = 0

	// BulkMaxPayloadSize is the maximum payload size for a Bulk request in bytes.
	// Zero because Bulk is not supported.
	BulkMaxPayloadSize = 0

	// FilterSupported indicates whether SCIM filtering is supported
	// per RFC 7644 §3.4.2.2.
	FilterSupported = true

	// FilterMaxResults caps the number of resources returned in a single
	// filtered query, guarding against excessively large result sets.
	FilterMaxResults = 200

	// ChangePasswordSupported indicates that the SCIM change-password
	// operation is not yet supported.
	ChangePasswordSupported = false

	// SortSupported indicates that SCIM result sorting is not yet supported
	// per RFC 7644 §3.4.2.3.
	SortSupported = false

	// ETagSupported indicates that ETag / versioning is supported
	// per RFC 7644 §3.14.
	ETagSupported = true
)

// SCIMConfig holds the SCIM service configuration resolved from the
// server runtime. All protocol capability flags are code-level constants
// above; this struct carries only the server-identity fields that must be
// read from the runtime environment.
type SCIMConfig struct {
	// PublicURL is the externally reachable base URL of the server,
	// used to construct SCIM resource location URIs.
	PublicURL string
}

// FromServerRuntime builds a SCIMConfig from the live server runtime.
// No SCIM-specific fields are read from the system config; all capability
// flags are defined as package-level constants above.
func FromServerRuntime() SCIMConfig {
	srv := config.GetServerRuntime().Config
	return SCIMConfig{
		PublicURL: srv.Server.PublicURL,
	}
}
