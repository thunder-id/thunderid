// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

// SCIMMeta holds SCIM resource metadata fields.
type SCIMMeta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Location     string `json:"location,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Created      string `json:"created,omitempty"`
	Version      string `json:"version,omitempty"`
}

// SCIMErrorResponse is the SCIM-standard error payload shape (RFC 7643 §3.12).
// This is what goes over the wire to SCIM clients — never internal error codes.
type SCIMErrorResponse struct {
	Schemas  []string      `json:"schemas"`
	Status   string        `json:"status"`
	ScimType ScimErrorType `json:"scimType,omitempty"`
	Detail   string        `json:"detail,omitempty"`
}

// SCIMSearchRequest is the request payload shape of the .search request
type SCIMSearchRequest struct {
	Schemas            []string `json:"schemas"`
	Filter             string   `json:"filter,omitempty"`
	Attributes         []string `json:"attributes,omitempty"`
	ExcludedAttributes []string `json:"excludedAttributes,omitempty"`
	SortBy             string   `json:"sortBy,omitempty"`
	SortOrder          string   `json:"sortOrder,omitempty"`
	StartIndex         int      `json:"startIndex,omitempty"`
	Count              *int     `json:"count,omitempty"`
}
