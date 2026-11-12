// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entitytype"
)

// SystemAttributes holds the system-level metadata of an organization unit type.
type SystemAttributes struct {
	Display string `json:"display,omitempty"`
}

// OUType represents an organization unit type.
type OUType struct {
	ID               string            `json:"id"`
	Handle           string            `json:"handle"`
	DisplayName      string            `json:"displayName"`
	OUID             string            `json:"ouId"`
	OUHandle         string            `json:"ouHandle,omitempty"`
	SystemAttributes *SystemAttributes `json:"systemAttributes,omitempty"`
	Schema           json.RawMessage   `json:"schema"`
	IsReadOnly       bool              `json:"isReadOnly"`
}

// OUTypeListItem represents an organization unit type in a list. A list omits the schema.
type OUTypeListItem struct {
	ID               string            `json:"id"`
	Handle           string            `json:"handle"`
	DisplayName      string            `json:"displayName"`
	OUID             string            `json:"ouId"`
	OUHandle         string            `json:"ouHandle,omitempty"`
	SystemAttributes *SystemAttributes `json:"systemAttributes,omitempty"`
	IsReadOnly       bool              `json:"isReadOnly"`
}

// OUTypeListResponse represents a page of organization unit types.
type OUTypeListResponse struct {
	TotalResults int               `json:"totalResults"`
	StartIndex   int               `json:"startIndex"`
	Count        int               `json:"count"`
	Types        []OUTypeListItem  `json:"types"`
	Links        []entitytype.Link `json:"links"`
}

// CreateOUTypeRequest represents the request body for creating an organization unit type.
type CreateOUTypeRequest struct {
	Handle           string            `json:"handle" native:"required,min=1,max=100"`
	DisplayName      string            `json:"displayName" native:"required,min=1"`
	OUID             string            `json:"ouId" native:"required"`
	SystemAttributes *SystemAttributes `json:"systemAttributes,omitempty"`
	Schema           json.RawMessage   `json:"schema" native:"required"`
}

// UpdateOUTypeRequest represents the request body for updating an organization unit type.
type UpdateOUTypeRequest struct {
	Handle           string            `json:"handle,omitempty"`
	DisplayName      string            `json:"displayName" native:"required,min=1"`
	OUID             string            `json:"ouId" native:"required"`
	SystemAttributes *SystemAttributes `json:"systemAttributes,omitempty"`
	Schema           json.RawMessage   `json:"schema"`
}
