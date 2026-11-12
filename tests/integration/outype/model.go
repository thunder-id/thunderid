// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import "encoding/json"

// OUType represents an organization unit type as returned by the detail endpoint.
type OUType struct {
	ID          string          `json:"id,omitempty"`
	Handle      string          `json:"handle,omitempty"`
	DisplayName string          `json:"displayName,omitempty"`
	OUID        string          `json:"ouId"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

// OUTypeRequest is the body for creating or updating an organization unit type.
type OUTypeRequest struct {
	Handle      string          `json:"handle,omitempty"`
	DisplayName string          `json:"displayName"`
	OUID        string          `json:"ouId"`
	Schema      json.RawMessage `json:"schema"`
}

// OUTypeListItem is a list entry. The list endpoint omits the schema.
type OUTypeListItem struct {
	ID          string `json:"id,omitempty"`
	Handle      string `json:"handle,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	OUID        string `json:"ouId"`
}

// OUTypeListResponse is the response from listing organization unit types.
type OUTypeListResponse struct {
	TotalResults int              `json:"totalResults"`
	Count        int              `json:"count"`
	Types        []OUTypeListItem `json:"types"`
}

// ErrorResponse is an API error response.
type ErrorResponse struct {
	Code string `json:"code"`
}
