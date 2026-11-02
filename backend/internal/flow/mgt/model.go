// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package flowmgt provides flow management data structures.
//
//nolint:lll
package flowmgt

import (
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// FlowDefinition represents the structure of a flow definition.
type FlowDefinition struct {
	ID           string                            `json:"id,omitempty"           yaml:"id,omitempty"           jsonschema:"Optional explicit ID for the flow. When omitted a UUID is generated."`
	Handle       string                            `json:"handle"                                               jsonschema:"Unique identifier for the flow (lowercase, alphanumeric with dashes/underscores). Example: 'basic-login', 'invite-registration'"          validate:"required"`
	Name         string                            `json:"name"                                                 jsonschema:"Display name for the flow. Example: 'Basic Login Flow', 'Invite Registration'"                                                            validate:"required"`
	FlowType     providers.FlowType                `json:"flowType"                                             jsonschema:"Type of flow: 'AUTHENTICATION' for login flows or 'REGISTRATION' for signup flows"                                                        validate:"required"`
	Interceptors []providers.InterceptorDefinition `json:"interceptors,omitempty" yaml:"interceptors,omitempty" jsonschema:"Optional array of interceptor declarations for cross-cutting concerns (e.g., CAPTCHA, rate limiting)."`
	Nodes        []providers.NodeDefinition        `json:"nodes"                                                jsonschema:"Array of nodes defining the flow steps. Must include START and END nodes. Use get_flow on existing flows to see node structure examples." validate:"required"`
	OUID         string                            `json:"ouId,omitempty"         yaml:"ouId,omitempty"         jsonschema:"Organization unit that owns the flow. Required on create and update, and cannot be changed afterwards."`
	OUHandle     string                            `json:"-"                      yaml:"-"`
}

// FlowDefinitionRequest represents the API request body for create/update flow operations.
// ID is intentionally excluded from API payloads.
type FlowDefinitionRequest struct {
	Handle       string                            `json:"handle"                 validate:"required"`
	Name         string                            `json:"name"                   validate:"required"`
	FlowType     providers.FlowType                `json:"flowType"               validate:"required"`
	Interceptors []providers.InterceptorDefinition `json:"interceptors,omitempty"`
	Nodes        []providers.NodeDefinition        `json:"nodes"                  validate:"required"`
	OUID         string                            `json:"ouId,omitempty"`
}

// BasicFlowDefinition represents basic information about a flow definition.
type BasicFlowDefinition struct {
	ID            string             `json:"id"            jsonschema:"Unique identifier of the flow."`
	Handle        string             `json:"handle"        jsonschema:"URL-friendly handle."`
	FlowType      providers.FlowType `json:"flowType"      jsonschema:"Type of flow (AUTHENTICATION or REGISTRATION)."`
	Name          string             `json:"name"          jsonschema:"Display name of the flow."`
	ActiveVersion int                `json:"activeVersion" jsonschema:"Current active version number."`
	CreatedAt     string             `json:"createdAt"     jsonschema:"Creation timestamp."`
	UpdatedAt     string             `json:"updatedAt"     jsonschema:"Last update timestamp."`
	IsReadOnly    bool               `json:"isReadOnly"    jsonschema:"Whether the flow is immutable (declarative)."`
	OUID          string             `json:"ouId"          jsonschema:"Organization unit that owns the flow."`
	IsOwnedByOU   *bool              `json:"isOwnedByOU,omitempty" jsonschema:"With an organization unit in the request: whether it owns the flow. False means the flow was shared with it."`
}

// OwnedFlow is a flow as the organization unit that owns it sees it: the whole definition.
type OwnedFlow struct {
	*providers.CompleteFlowDefinition
	IsOwnedByOU bool `json:"isOwnedByOU"`
}

// SharedFlow is a flow as an organization unit it was shared with sees it: its identity and the variables that apply
// to that organization unit, each resolved for it, and nothing else of the definition.
type SharedFlow struct {
	ID          string                         `json:"id"`
	Handle      string                         `json:"handle"`
	Name        string                         `json:"name"`
	FlowType    providers.FlowType             `json:"flowType"`
	OUID        string                         `json:"ouId"`
	IsOwnedByOU bool                           `json:"isOwnedByOU"`
	IsReadOnly  bool                           `json:"isReadOnly"`
	Variables   []providers.VariableDefinition `json:"variables"`
}

// FlowView is a flow as one organization unit sees it. Exactly one of the two is set.
type FlowView struct {
	Owned  *OwnedFlow
	Shared *SharedFlow
}

// VariablesResponse is the values an organization unit has set for the variables of a flow, keyed by identifier, in
// the variable's own type. A variable it has not set is absent.
type VariablesResponse struct {
	Values map[string]interface{} `json:"values"`
}

// SetVariablesRequest sets the values an organization unit holds for the variables of a flow. It replaces what the
// organization unit held: a variable left out returns to its default, or to the value a sharing policy fixes for it.
type SetVariablesRequest struct {
	OUID   string                 `json:"ouId"`
	Values map[string]interface{} `json:"values"`
}

// FlowListResponse represents a paginated list of flow definitions.
type FlowListResponse struct {
	TotalResults int                   `json:"totalResults" jsonschema:"Total number of flows available."`
	StartIndex   int                   `json:"startIndex"   jsonschema:"Starting index of the current page."`
	Count        int                   `json:"count"        jsonschema:"Number of flows in the current page."`
	Flows        []BasicFlowDefinition `json:"flows"        jsonschema:"List of flow definitions."`
	Links        []Link                `json:"links"        jsonschema:"Pagination links."`
}

// FlowVersion represents a specific version of a flow definition.
type FlowVersion struct {
	ID           string                            `json:"id"`
	Handle       string                            `json:"handle"`
	Name         string                            `json:"name"`
	FlowType     string                            `json:"flowType"`
	Version      int                               `json:"version"`
	IsActive     bool                              `json:"isActive"`
	Interceptors []providers.InterceptorDefinition `json:"interceptors,omitempty"`
	Nodes        []providers.NodeDefinition        `json:"nodes"`
	CreatedAt    string                            `json:"createdAt"`
}

// FlowVersionListResponse represents a list of flow versions.
type FlowVersionListResponse struct {
	TotalVersions int                `json:"totalVersions"`
	Versions      []BasicFlowVersion `json:"versions"`
}

// BasicFlowVersion represents basic information about a flow version.
type BasicFlowVersion struct {
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
	IsActive  bool   `json:"isActive"`
}

// RestoreVersionRequest represents a request to restore a specific version.
type RestoreVersionRequest struct {
	Version int `json:"version" validate:"required"`
}

// Link represents a hypermedia link for pagination.
type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// listFlowsInput represents the input for the list_flows tool.
type listFlowsInput struct {
	tool.PaginationInput
	FlowType string `json:"flow_type,omitempty" jsonschema:"Filter by flow type: 'AUTHENTICATION' for login flows or 'REGISTRATION' for signup flows. Omit to see all flows."`
}

// flowListOutput represents the output for list_flows tool.
type flowListOutput struct {
	TotalCount int                   `json:"total_count" jsonschema:"Total number of flows available."`
	Flows      []BasicFlowDefinition `json:"flows"       jsonschema:"List of flow definitions."`
}

// getFlowByHandleInput represents the input for get_flow_by_handle tool.
type getFlowByHandleInput struct {
	Handle   string `json:"handle"    jsonschema:"Flow handle to search for."`
	FlowType string `json:"flow_type" jsonschema:"Flow type: 'AUTHENTICATION' or 'REGISTRATION'. Required to uniquely identify the flow."`
}

// updateFlowInput represents the input for update_flow tool.
type updateFlowInput struct {
	ID           string                            `json:"id"                     jsonschema:"The unique identifier of the flow to update. Required."`
	Name         string                            `json:"name"                   jsonschema:"Display name for the flow. Required for PUT."`
	Nodes        []providers.NodeDefinition        `json:"nodes"                  jsonschema:"Array of nodes defining the flow steps. Required for PUT."`
	Interceptors []providers.InterceptorDefinition `json:"interceptors,omitempty" jsonschema:"Optional array of interceptor declarations for cross-cutting concerns."`
}

// CallTarget describes one flow reachable from a starting flow via CALL nodes.
type CallTarget struct {
	FlowID   string
	FlowType providers.FlowType
}
