// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package model defines the data transfer objects for the agent module.
//
//nolint:lll
package model

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// AgentRequestWithID is the YAML-aware struct used for declarative resource loading.
// Attributes uses map[string]interface{} (not json.RawMessage) so the YAML library can
// deserialize nested maps; the entity parser converts it to json.RawMessage before storage.
type AgentRequestWithID struct {
	ID          string                 `json:"id"                    yaml:"id"`
	OUID        string                 `json:"ouId,omitempty"        yaml:"ouId,omitempty"`
	OUHandle    string                 `json:"ouHandle,omitempty"    yaml:"ouHandle,omitempty"`
	Type        string                 `json:"type"                  yaml:"type"`
	Name        string                 `json:"name"                  yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	LogoURL     string                 `json:"logoUrl,omitempty"     yaml:"logoUrl,omitempty"`
	Owner       string                 `json:"owner,omitempty"       yaml:"owner,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"  yaml:"attributes,omitempty"`

	inboundmodel.InboundAuthProfileReq `yaml:",inline"`
	InboundAuthConfig                  []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" yaml:"inboundAuthConfig,omitempty"`
}

// Agent is the service-level model for agent create operations.
type Agent struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	// The service-level model carries the full internal profile, including SubjectAttribute, which
	// the API-facing request and response types deliberately exclude.
	providers.InboundAuthProfile
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// CreateAgentRequest is the HTTP request body for creating an agent.
type CreateAgentRequest struct {
	OUID        string          `json:"ouId" native:"required"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type" native:"required"`
	Name        string          `json:"name" native:"required,min=1,max=100"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfileReq
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// UpdateAgentRequest is the HTTP request body for updating an agent.
type UpdateAgentRequest struct {
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfileReq
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// AgentCompleteResponse is returned on create and update operations. Includes clientSecret
// in the embedded OAuth config when an OAuth profile was just provisioned.
type AgentCompleteResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfileReq
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// AgentGetResponse is returned on read operations. Excludes secrets (no clientSecret).
type AgentGetResponse struct {
	ID          string `json:"id,omitempty"          yaml:"id,omitempty"`
	OUID        string `json:"ouId,omitempty"        yaml:"ouId,omitempty"`
	OUHandle    string `json:"ouHandle,omitempty"    yaml:"-"`
	Type        string `json:"type,omitempty"        yaml:"type,omitempty"`
	Name        string `json:"name,omitempty"        yaml:"name,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"     yaml:"logoUrl,omitempty"`
	ClientID    string `json:"clientId,omitempty"    yaml:"-"`
	Owner       string `json:"owner,omitempty"       yaml:"owner,omitempty"`
	// Attributes holds the raw JSON for API responses; json.RawMessage cannot be
	// directly YAML-marshaled, so AttributesYAML carries the decoded map for export.
	Attributes     json.RawMessage        `json:"attributes,omitempty" yaml:"-"`
	AttributesYAML map[string]interface{} `json:"-"                    yaml:"attributes,omitempty"`

	inboundmodel.InboundAuthProfileReq `yaml:",inline"`
	InboundAuthConfig                  []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" yaml:"inboundAuthConfig,omitempty"`
}

// BasicAgentResponse is the summary view used in list responses.
type BasicAgentResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
	IsReadOnly  bool            `json:"isReadOnly"`
}

// AgentListResponse is the paginated list response.
type AgentListResponse struct {
	TotalResults int                  `json:"totalResults"`
	StartIndex   int                  `json:"startIndex"`
	Count        int                  `json:"count"`
	Agents       []BasicAgentResponse `json:"agents"`
	Links        []utils.Link         `json:"links"`
}

// AgentGroup is the group representation used in agent group list responses.
type AgentGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OUID string `json:"ouId"`
}

// AgentGroupListResponse is the paginated response for an agent's group memberships.
type AgentGroupListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Groups       []AgentGroup `json:"groups"`
	Links        []utils.Link `json:"links"`
}

// AgentRoleListResponse is the paginated response for an agent's assigned roles (direct and
// group-inherited). Roles are represented by name only, matching what the underlying role
// lookup returns.
type AgentRoleListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Roles        []string     `json:"roles"`
	Links        []utils.Link `json:"links"`
}
