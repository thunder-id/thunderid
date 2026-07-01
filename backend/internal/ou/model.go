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

	"github.com/thunder-id/thunderid/internal/system/utils"
)

// OrganizationUnitRequest represents the request body for creating an organization unit.
type OrganizationUnitRequest struct {
	Handle          string  `json:"handle"                    native:"required,min=3,max=50"`
	Name            string  `json:"name"                      native:"required,min=2,max=100"`
	Description     string  `json:"description,omitempty"`
	Parent          *string `json:"parent"                    native:"omitempty,max=255"`
	ThemeID         string  `json:"themeId,omitempty"`
	LayoutID        string  `json:"layoutId,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty"         native:"omitempty,url,max=2048"`
	TosURI          string  `json:"tosUri,omitempty"          native:"omitempty,url,max=2048"`
	PolicyURI       string  `json:"policyUri,omitempty"       native:"omitempty,url,max=2048"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty" native:"omitempty,url,max=2048"`
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
