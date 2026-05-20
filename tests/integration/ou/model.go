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
