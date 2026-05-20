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

package entitytype

import (
	"encoding/json"
)

// TypeCategory identifies the kind of entity that an entity type describes.
type TypeCategory string

const (
	// TypeCategoryUser categorizes schemas used to validate user entities.
	TypeCategoryUser TypeCategory = "user"
	// TypeCategoryAgent categorizes schemas used to validate agent entities.
	TypeCategoryAgent TypeCategory = "agent"
)

// DefaultAgentTypeName is the only agent type name allowed; agent types are restricted
// to a single 'default' schema.
const DefaultAgentTypeName = "default"

// IsValid reports whether the category is one of the known fixed values.
func (c TypeCategory) IsValid() bool {
	return c == TypeCategoryUser || c == TypeCategoryAgent
}

// Note: Complex JSON schema type definitions (array, boolean, number, object, schema, string)
// are kept in the model/ subdirectory to maintain clean separation and better organization.
// This file contains only the simple DTOs and API request/response structures.

// SystemAttributes holds system-level metadata for an entity type.
// Stored as a JSON column for extensibility — new fields can be added without DB migrations.
type SystemAttributes struct {
	Display string `json:"display,omitempty" yaml:"display,omitempty"`
}

// EntityType represents an entity-type schema definition.
type EntityType struct {
	ID                    string            `json:"id,omitempty" yaml:"id,omitempty"`
	Category              TypeCategory      `json:"-" yaml:"category,omitempty"`
	Name                  string            `json:"name,omitempty" yaml:"name"`
	OUID                  string            `json:"ouId" yaml:"organization_unit_id"`
	OUHandle              string            `json:"ouHandle,omitempty" yaml:"-"`
	AllowSelfRegistration bool              `json:"allowSelfRegistration" yaml:"allow_self_registration,omitempty"`
	SystemAttributes      *SystemAttributes `json:"systemAttributes,omitempty" yaml:"system_attributes,omitempty"`
	Schema                json.RawMessage   `json:"schema,omitempty" yaml:"schema"`
}

// EntityTypeListItem represents a simplified entity type for listing operations.
// Category is internal — see EntityType for the rationale.
type EntityTypeListItem struct {
	ID                    string            `json:"id,omitempty"`
	Category              TypeCategory      `json:"-"`
	Name                  string            `json:"name,omitempty"`
	OUID                  string            `json:"ouId"`
	OUHandle              string            `json:"ouHandle,omitempty"`
	AllowSelfRegistration bool              `json:"allowSelfRegistration"`
	SystemAttributes      *SystemAttributes `json:"systemAttributes,omitempty"`
	IsReadOnly            bool              `json:"isReadOnly"`
}

// Link represents a hypermedia link in the API response.
type Link struct {
	Href string `json:"href,omitempty"`
	Rel  string `json:"rel,omitempty"`
}

// EntityTypeListResponse represents the response for listing entity types with pagination.
type EntityTypeListResponse struct {
	TotalResults int                  `json:"totalResults"`
	StartIndex   int                  `json:"startIndex"`
	Count        int                  `json:"count"`
	Types        []EntityTypeListItem `json:"types"`
	Links        []Link               `json:"links"`
}

// CreateEntityTypeRequest represents the request body for creating an entity type.
type CreateEntityTypeRequest struct {
	Name                  string            `json:"name"`
	OUID                  string            `json:"ouId"`
	AllowSelfRegistration bool              `json:"allowSelfRegistration,omitempty"`
	SystemAttributes      *SystemAttributes `json:"systemAttributes,omitempty"`
	Schema                json.RawMessage   `json:"schema"`
}

// CreateEntityTypeRequestWithID represents the service-level request for creating an entity type,
// including an optional ID.
type CreateEntityTypeRequestWithID struct {
	ID                    string            `json:"id,omitempty" yaml:"id,omitempty"`
	Name                  string            `json:"name"`
	OUID                  string            `json:"ouId"`
	OUHandle              string            `json:"ouHandle,omitempty"`
	AllowSelfRegistration bool              `json:"allowSelfRegistration,omitempty"`
	SystemAttributes      *SystemAttributes `json:"systemAttributes,omitempty"`
	Schema                json.RawMessage   `json:"schema"`
}

// UpdateEntityTypeRequest represents the request body for updating an entity type.
type UpdateEntityTypeRequest struct {
	Name                  string            `json:"name"`
	OUID                  string            `json:"ouId"`
	OUHandle              string            `json:"ouHandle,omitempty"`
	AllowSelfRegistration bool              `json:"allowSelfRegistration,omitempty"`
	SystemAttributes      *SystemAttributes `json:"systemAttributes,omitempty"`
	Schema                json.RawMessage   `json:"schema"`
}

// EntityTypeRequestWithID represents the request structure for creating an entity type from
// file-based config.
type EntityTypeRequestWithID struct {
	ID                    string            `yaml:"id"`
	Category              TypeCategory      `yaml:"category,omitempty"`
	Name                  string            `yaml:"name"`
	OUID                  string            `yaml:"organization_unit_id,omitempty"`
	OUHandle              string            `yaml:"ou_handle,omitempty"`
	AllowSelfRegistration bool              `yaml:"allow_self_registration,omitempty"`
	SystemAttributes      *SystemAttributes `yaml:"system_attributes,omitempty"`
	Schema                interface{}       `yaml:"schema"`
}
