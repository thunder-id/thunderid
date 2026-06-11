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

package resource

// ResourceServerType represents the type of a resource server.
type ResourceServerType string

const (
	// ResourceServerTypeAPI represents an API resource server.
	ResourceServerTypeAPI ResourceServerType = "API"
	// ResourceServerTypeMCP represents an MCP resource server.
	ResourceServerTypeMCP ResourceServerType = "MCP"
	// ResourceServerTypeCustom represents a custom resource server.
	ResourceServerTypeCustom ResourceServerType = "CUSTOM"
)

// supportedResourceServerTypes lists all the supported resource server types.
var supportedResourceServerTypes = []ResourceServerType{
	ResourceServerTypeAPI,
	ResourceServerTypeMCP,
	ResourceServerTypeCustom,
}

// IsValid reports whether the resource server type is one of the supported values.
func (t ResourceServerType) IsValid() bool {
	for _, supported := range supportedResourceServerTypes {
		if t == supported {
			return true
		}
	}
	return false
}

// HTTP Response Models

// ResourceServerResponse represents a resource server.
type ResourceServerResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Handle      string             `json:"handle"`
	Identifier  string             `json:"identifier,omitempty"`
	Type        ResourceServerType `json:"type"`
	OUID        string             `json:"ouId"`
	Delimiter   string             `json:"delimiter"`
	IsReadOnly  bool               `json:"isReadOnly"`
}

// ResourceResponse represents a resource.
type ResourceResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Handle      string  `json:"handle"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent,omitempty"`
	Permission  string  `json:"permission"`
}

// ActionResponse represents an action.
type ActionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Handle      string `json:"handle"`
	Description string `json:"description,omitempty"`
	Permission  string `json:"permission"`
}

// LinkResponse represents a pagination link.
type LinkResponse struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// ResourceServerListResponse represents the response for listing resource servers.
type ResourceServerListResponse struct {
	TotalResults    int                      `json:"totalResults"`
	StartIndex      int                      `json:"startIndex"`
	Count           int                      `json:"count"`
	ResourceServers []ResourceServerResponse `json:"resourceServers"`
	Links           []LinkResponse           `json:"links"`
}

// ResourceListResponse represents the response for listing resources.
type ResourceListResponse struct {
	TotalResults int                `json:"totalResults"`
	StartIndex   int                `json:"startIndex"`
	Count        int                `json:"count"`
	Resources    []ResourceResponse `json:"resources"`
	Links        []LinkResponse     `json:"links"`
}

// ActionListResponse represents the response for listing actions.
type ActionListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	Count        int              `json:"count"`
	Actions      []ActionResponse `json:"actions"`
	Links        []LinkResponse   `json:"links"`
}

// CreateResourceServerRequest represents the request to create a resource server.
type CreateResourceServerRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Handle      string             `json:"handle,omitempty"`
	Identifier  string             `json:"identifier,omitempty"`
	Type        ResourceServerType `json:"type,omitempty"`
	OUID        string             `json:"ouId"`
	Delimiter   string             `json:"delimiter,omitempty"`
}

// UpdateResourceServerRequest represents the request to update a resource server.
type UpdateResourceServerRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Handle      string `json:"handle,omitempty"`
	Identifier  string `json:"identifier,omitempty"`
	OUID        string `json:"ouId"`
}

// CreateResourceRequest represents the request to create a resource.
type CreateResourceRequest struct {
	Name        string  `json:"name"`
	Handle      string  `json:"handle"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent"`
}

// UpdateResourceRequest represents the request to update a resource.
type UpdateResourceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateActionRequest represents the request to create an action.
type CreateActionRequest struct {
	Name        string `json:"name"`
	Handle      string `json:"handle"`
	Description string `json:"description,omitempty"`
}

// UpdateActionRequest represents the request to update an action.
type UpdateActionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Link represents a pagination link in the service layer.
type Link struct {
	Href string
	Rel  string
}

// ResourceServerList represents the result of listing resource servers.
type ResourceServerList struct {
	TotalResults    int
	StartIndex      int
	Count           int
	ResourceServers []ResourceServer
	Links           []Link
}

// ResourceList represents the result of listing resources.
type ResourceList struct {
	TotalResults int
	StartIndex   int
	Count        int
	Resources    []Resource
	Links        []Link
}

// ActionList represents the result of listing actions.
type ActionList struct {
	TotalResults int
	StartIndex   int
	Count        int
	Actions      []Action
	Links        []Link
}

// Consolidated resource models for YAML parsing, processing, and service layer
// These models use:
// - yaml tags for YAML parsing (serialize/deserialize)
// - json tags for many fields (e.g., in Action, Resource, ResourceServer) for service/API use
// - Computed/internal fields marked with json:"-" and yaml:"-" as appropriate

// Action represents an action in both declarative resources and service layer.
type Action struct {
	ID          string `yaml:"-" json:"-"` // Set when retrieved from database
	Name        string `yaml:"name" json:"name"`
	Handle      string `yaml:"handle" json:"handle"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Permission  string `yaml:"-" json:"-"` // Computed permission string, not serialized to YAML
}

// Resource represents a resource in both declarative resources and service layer.
type Resource struct {
	ID           string   `yaml:"-" json:"-"` // Set when retrieved from database
	Name         string   `yaml:"name" json:"name"`
	Handle       string   `yaml:"handle" json:"handle"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	Parent       *string  `yaml:"-" json:"-"`                               // Resolved parent ID
	ParentHandle string   `yaml:"parent,omitempty" json:"parent,omitempty"` // Parent handle during YAML parsing only
	Permission   string   `yaml:"-" json:"-"`                               // Computed permission string
	Actions      []Action `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// ResourceServer represents a resource server in both declarative resources and service layer.
type ResourceServer struct {
	ID          string             `yaml:"id" json:"-"`
	Name        string             `yaml:"name" json:"name"`
	Description string             `yaml:"description,omitempty" json:"description,omitempty"`
	Handle      string             `yaml:"handle" json:"handle"`
	Identifier  string             `yaml:"identifier,omitempty" json:"identifier,omitempty"`
	Type        ResourceServerType `yaml:"type,omitempty" json:"type,omitempty"`
	OUID        string             `yaml:"ou_id,omitempty" json:"ouId"`
	OUHandle    string             `yaml:"ou_handle,omitempty" json:"-"`
	Delimiter   string             `yaml:"delimiter,omitempty" json:"delimiter,omitempty" yamlfmt:"quoted"`
	IsReadOnly  bool               `yaml:"-" json:"-"`
	Resources   []Resource         `yaml:"resources,omitempty" json:"resources,omitempty"`
}
