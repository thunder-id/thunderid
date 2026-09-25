// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package scimconfig provides the SCIM service configuration.
package scimconfig

import (
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// SCIMConfig holds the SCIM service configuration resolved from the server runtime.
//
// The capability fields describe what this server implementation supports. They are facts
// about the codebase, not deployment decisions, so they are not operator-configurable.
type SCIMConfig struct {
	// PublicURL is the externally reachable base URL of the server,
	// used to construct SCIM resource location URIs.
	PublicURL string

	// CoreUserTypeID designates the ThunderID user type whose schema backs the SCIM
	// core User schema. See system/config.SCIMConfig.CoreUserTypeID for details.
	// GET responses (GetUser, ListUsers) include core schema fields (userName, emails,
	// name, etc.) mapped from stored attributes only for users of this type; users of
	// any other type return their custom extension schema only.
	CoreUserTypeID string

	// SchemaURNPrefix is the URN prefix of the custom per-user-type SCIM schemas, ending in
	// a colon. The user type name and ":2.0:User" are appended to it.
	SchemaURNPrefix string

	// PatchSupported is reported in ServiceProviderConfig as the SCIM PATCH capability
	// (RFC 7644 §3.5.2). It is false because PATCH is only implemented for Groups; PATCH on
	// Users is not implemented yet and returns 501.
	PatchSupported bool

	// BulkSupported indicates whether SCIM Bulk operations are supported
	// (RFC 7644 §3.7). Not yet implemented.
	BulkSupported bool

	// BulkMaxOperations is the maximum number of operations in a Bulk request.
	// Zero because Bulk is not supported.
	BulkMaxOperations int

	// BulkMaxPayloadSize is the maximum payload size for a Bulk request in bytes.
	// Zero because Bulk is not supported.
	BulkMaxPayloadSize int

	// FilterSupported indicates whether SCIM filtering is supported
	// (RFC 7644 §3.4.2.2).
	FilterSupported bool

	// FilterMaxResults caps the number of resources returned in a single
	// filtered query, guarding against excessively large result sets.
	FilterMaxResults int

	// ChangePasswordSupported indicates whether the SCIM change-password
	// operation is supported. Not yet implemented.
	ChangePasswordSupported bool

	// SortSupported indicates whether SCIM result sorting is supported
	// (RFC 7644 §3.4.2.3). Not yet implemented.
	SortSupported bool

	// ETagSupported indicates whether ETag / versioning is supported (RFC 7644 §3.14).
	// Resource versions are not persisted at the database layer, so there is no way to
	// detect concurrent writes; advertising ETag support without real optimistic
	// concurrency control would be misleading to clients.
	ETagSupported bool

	// PaginationCursorSupported indicates whether cursor-based pagination
	// (RFC 9865) is supported. Not implemented; this server only supports
	// index-based pagination.
	PaginationCursorSupported bool

	// PaginationIndexSupported indicates whether index-based pagination
	// (startIndex/count, RFC 7644 §3.4.2.4) is supported.
	PaginationIndexSupported bool

	// PaginationDefaultMethod is the pagination method used when a client
	// does not specify a preference. Must be "cursor" or "index" per RFC 9865.
	PaginationDefaultMethod string

	// PaginationDefaultPageSize is the default number of resources returned
	// per page when a client does not specify "count".
	PaginationDefaultPageSize int

	// PaginationMaxPageSize is the maximum number of resources returned
	// per page, regardless of the requested "count".
	PaginationMaxPageSize int
}

// DefaultSchemaURNPrefix is used when no custom schema URN prefix is configured.
const DefaultSchemaURNPrefix = "urn:thunderid:params:scim:schemas:"

// schemaURNPrefix returns the configured prefix, defaulting when empty and ensuring a trailing colon.
func schemaURNPrefix(configured string) string {
	prefix := strings.TrimSpace(configured)
	if prefix == "" {
		return DefaultSchemaURNPrefix
	}
	if !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return prefix
}

// FromServerRuntime builds a SCIMConfig from the live server runtime.
func FromServerRuntime() SCIMConfig {
	srv := config.GetServerRuntime().Config
	return SCIMConfig{
		PublicURL:                 engineconfig.GetServerURL(&srv.Server),
		CoreUserTypeID:            srv.SCIM.CoreUserTypeID,
		SchemaURNPrefix:           schemaURNPrefix(srv.SCIM.SchemaURNPrefix),
		PatchSupported:            false,
		BulkSupported:             false,
		BulkMaxOperations:         0,
		BulkMaxPayloadSize:        0,
		FilterSupported:           true,
		FilterMaxResults:          serverconst.MaxPageSize,
		ChangePasswordSupported:   false,
		SortSupported:             false,
		ETagSupported:             false,
		PaginationCursorSupported: false,
		PaginationIndexSupported:  true,
		PaginationDefaultMethod:   "index",
		PaginationDefaultPageSize: serverconst.DefaultPageSize,
		PaginationMaxPageSize:     serverconst.MaxPageSize,
	}
}
