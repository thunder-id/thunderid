// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package scimconfig provides the SCIM service configuration.
package scimconfig

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

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
	FilterMaxResults = serverconst.MaxPageSize

	// ChangePasswordSupported indicates that the SCIM change-password
	// operation is not yet supported.
	ChangePasswordSupported = false

	// SortSupported indicates that SCIM result sorting is not yet supported
	// per RFC 7644 §3.4.2.3.
	SortSupported = false

	// ETagSupported indicates that ETag / versioning is supported
	// per RFC 7644 §3.14.
	ETagSupported = true

	// PaginationCursorSupported indicates whether cursor-based pagination
	// (RFC 9865) is supported. Not implemented; this server only supports
	// index-based pagination.
	PaginationCursorSupported = false

	// PaginationIndexSupported indicates whether index-based pagination
	// (startIndex/count, RFC 7644 §3.4.2.4) is supported.
	PaginationIndexSupported = true

	// PaginationDefaultMethod is the pagination method used when a client
	// does not specify a preference. Must be "cursor" or "index" per RFC 9865.
	PaginationDefaultMethod = "index"

	// PaginationDefaultPageSize is the default number of resources returned
	// per page when a client does not specify "count".
	PaginationDefaultPageSize = serverconst.DefaultPageSize

	// PaginationMaxPageSize is the maximum number of resources returned
	// per page, regardless of the requested "count".
	PaginationMaxPageSize = serverconst.MaxPageSize
)

// SCIMConfig holds the SCIM service configuration resolved from the
// server runtime. Protocol capability flags that are not operator-configurable
// are the code-level constants above; this struct carries the fields that
// are read from the runtime environment.
type SCIMConfig struct {
	// PublicURL is the externally reachable base URL of the server,
	// used to construct SCIM resource location URIs.
	PublicURL string

	// ReturnMappedCoreAttrsOnGet controls whether GET responses (GetUser,
	// ListUsers) include core schema fields (userName, emails, name, etc.)
	// mapped from stored attributes, or only the custom extension schema.
	// RFC 7644 expects the full resource representation on GET.
	ReturnMappedCoreAttrsOnGet bool
}

// FromServerRuntime builds a SCIMConfig from the live server runtime.
func FromServerRuntime() SCIMConfig {
	srv := config.GetServerRuntime().Config
	var returnCoreAttrs bool
	if srv.SCIM.ReturnMappedCoreAttrsOnGet != nil {
		returnCoreAttrs = *srv.SCIM.ReturnMappedCoreAttrsOnGet
	}
	return SCIMConfig{
		PublicURL:                  engineconfig.GetServerURL(&srv.Server),
		ReturnMappedCoreAttrsOnGet: returnCoreAttrs,
	}
}
