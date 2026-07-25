/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package layoutmgt

import "encoding/json"

// Layout represents a layout configuration.
type Layout struct {
	ID          string          `json:"id" yaml:"id,omitempty"`
	Handle      string          `json:"handle" yaml:"handle"`
	DisplayName string          `json:"displayName" yaml:"displayName"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
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
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// CreateLayoutRequest represents the request body for creating a layout configuration.
type CreateLayoutRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// CreateLayoutRequestWithID represents the service-level request for creating a layout, including an optional ID.
type CreateLayoutRequestWithID struct {
	ID          string          `json:"id,omitempty" yaml:"id,omitempty"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// UpdateLayoutRequest represents the request body for updating a layout configuration.
type UpdateLayoutRequest struct {
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Layout      json.RawMessage `json:"layout"`
}

// layoutRequestWithID represents the request structure for creating a layout from file-based config.
type layoutRequestWithID struct {
	ID          string      `yaml:"id"`
	Handle      string      `yaml:"handle"`
	DisplayName string      `yaml:"displayName"`
	Description string      `yaml:"description,omitempty"`
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
