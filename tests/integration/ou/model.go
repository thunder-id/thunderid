// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// OrganizationUnitBasic represents the basic information of an organization unit.
type OrganizationUnitBasic struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	LogoURL     string    `json:"logoUrl,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OrganizationUnit represents an organization unit.
type OrganizationUnit struct {
	ID              string    `json:"id"`
	Handle          string    `json:"handle"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Parent          *string   `json:"parent"`
	LogoURL         string    `json:"logoUrl,omitempty"`
	TosURI          string    `json:"tosUri,omitempty"`
	PolicyURI       string    `json:"policyUri,omitempty"`
	CookiePolicyURI string    `json:"cookiePolicyUri,omitempty"`
	ThemeID         string    `json:"themeId,omitempty"`
	LayoutID        string    `json:"layoutId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// CreateOURequest represents the request body for creating an organization unit.
type CreateOURequest struct {
	Handle          string  `json:"handle"`
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	Parent          *string `json:"parent,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty"`
	TosURI          string  `json:"tosUri,omitempty"`
	PolicyURI       string  `json:"policyUri,omitempty"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty"`
	ThemeID         string  `json:"themeId,omitempty"`
	LayoutID        string  `json:"layoutId,omitempty"`
}

// UpdateOURequest represents the request body for updating an organization unit.
type UpdateOURequest struct {
	Handle          string  `json:"handle"`
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	Parent          *string `json:"parent,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty"`
	TosURI          string  `json:"tosUri,omitempty"`
	PolicyURI       string  `json:"policyUri,omitempty"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty"`
	ThemeID         string  `json:"themeId,omitempty"`
	LayoutID        string  `json:"layoutId,omitempty"`
}

// OrganizationUnitListResponse represents the response for listing organization units with pagination.
type OrganizationUnitListResponse struct {
	TotalResults      int                     `json:"totalResults"`
	StartIndex        int                     `json:"startIndex"`
	Count             int                     `json:"count"`
	OrganizationUnits []OrganizationUnitBasic `json:"organizationUnits"`
	Links             []testutils.Link        `json:"links"`
}

// User represents a user with basic information for OU endpoints.
type User struct {
	ID string `json:"id"`
}

// Group represents a group with basic information for OU endpoints.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GroupListResponse represents the response for listing groups in an organization unit.
type GroupListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Groups       []Group          `json:"groups"`
	Links        []testutils.Link `json:"links"`
}

// Role represents a role with basic information for OU endpoints.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// RoleListResponse represents the response for listing roles in an organization unit.
type RoleListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Roles        []Role           `json:"roles"`
	Links        []testutils.Link `json:"links"`
}

type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description,omitempty"`
}
