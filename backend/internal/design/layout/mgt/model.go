// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

import "encoding/json"

// Layout represents a layout configuration.
type Layout struct {
	ID          string          `json:"id" yaml:"id,omitempty"`
	Handle      string          `json:"handle" yaml:"handle"`
	DisplayName string          `json:"displayName" yaml:"displayName"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	ProjectID   string          `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Layout      json.RawMessage `json:"layout" yaml:"layout"`
	CreatedAt   string          `json:"createdAt" yaml:"createdAt,omitempty"`
	UpdatedAt   string          `json:"updatedAt" yaml:"updatedAt,omitempty"`
	IsReadOnly  bool            `json:"isReadOnly" yaml:"isReadOnly"`
}

// LayoutListItem represents a layout item in the list response.
type LayoutListItem struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// CreateLayoutRequest represents the request body for creating a layout configuration.
type CreateLayoutRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	ProjectID   string          `json:"projectId,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// CreateLayoutRequestWithID represents the service-level request for creating a layout, including an optional ID.
type CreateLayoutRequestWithID struct {
	ID          string          `json:"id,omitempty" yaml:"id,omitempty"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	ProjectID   string          `json:"projectId,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// UpdateLayoutRequest represents the request body for updating a layout configuration.
type UpdateLayoutRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	ProjectID   string          `json:"projectId,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// layoutRequestWithID represents the request structure for creating a layout from file-based config.
type layoutRequestWithID struct {
	ID          string      `yaml:"id"`
	Handle      string      `yaml:"handle"`
	DisplayName string      `yaml:"displayName"`
	Description string      `yaml:"description,omitempty"`
	ProjectID   string      `yaml:"projectId,omitempty"`
	Layout      interface{} `yaml:"layout"`
}

// LayoutListResponse represents the response for listing layout configurations with pagination.
type LayoutListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Layouts      []LayoutListItem `json:"layouts"`
	Links        []LinkResponse   `json:"links"`
}

// LayoutListResponseWithFullLayouts represents the response for listing layout configurations with full layout data.
type LayoutListResponseWithFullLayouts struct {
	TotalResults int            `json:"totalResults"`
	StartIndex   int            `json:"startIndex"`
	Count        int            `json:"count"`
	Layouts      []Layout       `json:"layouts"`
	Links        []LinkResponse `json:"links"`
}

// LinkResponse represents a pagination link.
type LinkResponse struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// Link represents a pagination link.
type Link struct {
	Href string
	Rel  string
}

// LayoutList represents the result of listing layout configurations.
type LayoutList struct {
	TotalResults int
	StartIndex   int
	Count        int
	Layouts      []Layout
	Links        []Link
}
