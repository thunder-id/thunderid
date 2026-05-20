/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package ou

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// OrganizationUnitBasic represents the basic information of an organization unit.
type OrganizationUnitBasic struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	LogoURL     string    `json:"logoUrl,omitempty"`
	IsReadOnly  bool      `json:"isReadOnly"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OrganizationUnit represents an organization unit.
type OrganizationUnit struct {
	ID              string    `json:"id" yaml:"id"`
	Handle          string    `json:"handle" yaml:"handle"`
	Name            string    `json:"name" yaml:"name"`
	Description     string    `json:"description,omitempty" yaml:"description,omitempty"`
	Parent          *string   `json:"parent" yaml:"parent"`
	ThemeID         string    `json:"themeId,omitempty" yaml:"theme_id,omitempty"`
	LayoutID        string    `json:"layoutId,omitempty" yaml:"layout_id,omitempty"`
	LogoURL         string    `json:"logoUrl,omitempty" yaml:"logo_url,omitempty"`
	TosURI          string    `json:"tosUri,omitempty" yaml:"tos_uri,omitempty"`
	PolicyURI       string    `json:"policyUri,omitempty" yaml:"policy_uri,omitempty"`
	CookiePolicyURI string    `json:"cookiePolicyUri,omitempty" yaml:"cookie_policy_uri,omitempty"`
	CreatedAt       time.Time `json:"createdAt" yaml:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" yaml:"updated_at"`
}

// OrganizationUnitRequest represents the request body for creating an organization unit.
type OrganizationUnitRequest struct {
	Handle          string  `json:"handle"`
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	Parent          *string `json:"parent"`
	ThemeID         string  `json:"themeId,omitempty"`
	LayoutID        string  `json:"layoutId,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty"`
	TosURI          string  `json:"tosUri,omitempty"`
	PolicyURI       string  `json:"policyUri,omitempty"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty"`
}

// OrganizationUnitRequestWithID represents the request body for creating an organization unit
// in import/declarative paths where preserving IDs is required.
type OrganizationUnitRequestWithID struct {
	ID              string  `json:"id" yaml:"id"`
	Handle          string  `json:"handle" yaml:"handle"`
	Name            string  `json:"name" yaml:"name"`
	Description     string  `json:"description,omitempty" yaml:"description,omitempty"`
	Parent          *string `json:"parent" yaml:"parent"`
	ThemeID         string  `json:"themeId,omitempty" yaml:"theme_id,omitempty"`
	LayoutID        string  `json:"layoutId,omitempty" yaml:"layout_id,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty" yaml:"logo_url,omitempty"`
	TosURI          string  `json:"tosUri,omitempty" yaml:"tos_uri,omitempty"`
	PolicyURI       string  `json:"policyUri,omitempty" yaml:"policy_uri,omitempty"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty" yaml:"cookie_policy_uri,omitempty"`
}

// OrganizationUnitListResponse represents the response for listing organization units with pagination.
type OrganizationUnitListResponse struct {
	TotalResults      int                     `json:"totalResults"`
	StartIndex        int                     `json:"startIndex"`
	Count             int                     `json:"count"`
	OrganizationUnits []OrganizationUnitBasic `json:"organizationUnits"`
	Links             []utils.Link            `json:"links"`
}

// User represents a user with basic information for OU endpoints.
type User struct {
	ID      string `json:"id"`
	Type    string `json:"type,omitempty"`
	Display string `json:"display,omitempty"`
}

// Group represents a group with basic information for OU endpoints.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserListResponse represents the response for listing users in an organization unit.
type UserListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Users        []User       `json:"users"`
	Links        []utils.Link `json:"links"`
}

// OUUserResolver provides access to user data for an organization unit
// without requiring direct import of the user package.
type OUUserResolver interface {
	GetUserCountByOUID(ctx context.Context, ouID string) (int, error)
	GetUserListByOUID(ctx context.Context, ouID string, limit, offset int, includeDisplay bool) ([]User, error)
}

// OUGroupResolver provides access to group data for an organization unit
// without requiring direct import of the group package.
type OUGroupResolver interface {
	GetGroupCountByOUID(ctx context.Context, ouID string) (int, error)
	GetGroupListByOUID(ctx context.Context, ouID string, limit, offset int) ([]Group, error)
}

// GroupListResponse represents the response for listing groups in an organization unit.
type GroupListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Groups       []Group      `json:"groups"`
	Links        []utils.Link `json:"links"`
}
