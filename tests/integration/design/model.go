// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package design

import "encoding/json"

type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// CreateThemeRequest represents the request payload for creating a theme.
type CreateThemeRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Theme       json.RawMessage `json:"theme"`
}

// UpdateThemeRequest represents the request payload for updating a theme.
type UpdateThemeRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Theme       json.RawMessage `json:"theme"`
}

// ThemeResponse represents a theme response.
type ThemeResponse struct {
	ID          string          `json:"id"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Theme       json.RawMessage `json:"theme"`
}

// ThemeListItem represents a theme in the list response.
type ThemeListItem struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

// ThemeListResponse represents the response for listing themes.
type ThemeListResponse struct {
	TotalResults int             `json:"totalResults"`
	StartIndex   int             `json:"startIndex"`
	Count        int             `json:"count"`
	Themes       []ThemeListItem `json:"themes"`
	Links        []Link          `json:"links,omitempty"`
}

// CreateLayoutRequest represents the request payload for creating a layout.
type CreateLayoutRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// UpdateLayoutRequest represents the request payload for updating a layout.
type UpdateLayoutRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// LayoutResponse represents a layout response.
type LayoutResponse struct {
	ID          string          `json:"id"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// LayoutListItem represents a layout in the list response.
type LayoutListItem struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

// LayoutListResponse represents the response for listing layouts.
type LayoutListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Layouts      []LayoutListItem `json:"layouts"`
	Links        []Link           `json:"links,omitempty"`
}

// DesignResolveResponse represents the response for design resolve endpoint.
type DesignResolveResponse struct {
	Theme  json.RawMessage `json:"theme,omitempty"`
	Layout json.RawMessage `json:"layout,omitempty"`
}

// Link represents a pagination link.
type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// ResourceDependency represents a single resource that references a theme or layout.
type ResourceDependency struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// DependenciesResponse represents the response of the theme/layout usages endpoints. TotalResults
// and Summary are nil when dependency data is unavailable, distinct from a confirmed-empty result.
type DependenciesResponse struct {
	TotalResults *int                 `json:"totalResults"`
	Count        int                  `json:"count"`
	Summary      map[string]int       `json:"summary"`
	Usages       []ResourceDependency `json:"usages"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description,omitempty"`
}
