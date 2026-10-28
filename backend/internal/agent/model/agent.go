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

// AgentSystemAttributes holds the agent fields kept in the entity SystemAttributes blob.
type AgentSystemAttributes struct {
	Description string
	Owner       string
	ClientID    string
	Kind        providers.AgentKind
}

// AgentRequestWithID is the YAML-aware struct used for declarative resource loading.
// Attributes uses map[string]interface{} (not json.RawMessage) so the YAML library can
// deserialize nested maps; the entity parser converts it to json.RawMessage before storage.
type AgentRequestWithID struct {
	ID          string                 `json:"id"                    yaml:"id"`
	OUID        string                 `json:"ouId,omitempty"        yaml:"ouId,omitempty"`
	OUHandle    string                 `json:"ouHandle,omitempty"    yaml:"ouHandle,omitempty"`
	Type        string                 `json:"type"                  yaml:"type"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	LogoURL     string                 `json:"logoUrl,omitempty"     yaml:"logoUrl,omitempty"`
	Owner       string                 `json:"owner,omitempty"       yaml:"owner,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"  yaml:"attributes,omitempty"`
	Kind        providers.AgentKind    `json:"kind,omitempty" yaml:"kind,omitempty"`

	inboundmodel.InboundAuthProfileReq `yaml:",inline"`
	InboundAuthConfig                  []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" yaml:"inboundAuthConfig,omitempty"`
}

// CreateAgentRequest is the HTTP request body for creating an agent.
type CreateAgentRequest struct {
	OUID        string          `json:"ouId" native:"required"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type" native:"required"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Delegated   bool            `json:"delegated,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	Kind providers.AgentKind `json:"kind,omitempty"`

	inboundmodel.InboundAuthProfileReq
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// UpdateAgentRequest is the HTTP request body for updating an agent.
type UpdateAgentRequest struct {
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Delegated   bool            `json:"delegated,omitempty"`
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
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
	// Status is the effective access status and what produced it, set on update responses.
	Status *providers.AccessStatusReport `json:"status,omitempty"`
	// LastLoginAt is when this agent was last granted access, set on update responses.
	LastLoginAt string `json:"lastLoginAt,omitempty"`

	Kind providers.AgentKind `json:"kind,omitempty"`

	inboundmodel.InboundAuthProfileReq
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// AgentGetResponse is returned on read operations. Excludes secrets (no clientSecret).
type AgentGetResponse struct {
	ID          string `json:"id,omitempty"          yaml:"id,omitempty"`
	OUID        string `json:"ouId,omitempty"        yaml:"ouId,omitempty"`
	OUHandle    string `json:"ouHandle,omitempty"    yaml:"-"`
	Type        string `json:"type,omitempty"        yaml:"type,omitempty"`
	Display     string `json:"display,omitempty"     yaml:"-"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"     yaml:"logoUrl,omitempty"`
	ClientID    string `json:"clientId,omitempty"    yaml:"-"`
	Owner       string `json:"owner,omitempty"       yaml:"owner,omitempty"`
	// Attributes holds the raw JSON for API responses; json.RawMessage cannot be
	// directly YAML-marshaled, so AttributesYAML carries the decoded map for export.
	Attributes     json.RawMessage        `json:"attributes,omitempty" yaml:"-"`
	AttributesYAML map[string]interface{} `json:"-"                    yaml:"attributes,omitempty"`
	// Status is the effective access status and what produced it, derived at read time: SUSPENDED
	// beats LOCKED beats ACTIVE. Read-only, and excluded from the YAML export as runtime state.
	Status *providers.AccessStatusReport `json:"status,omitempty" yaml:"-"`
	// LastLoginAt is when this agent was last granted access. Recorded by the runtime, never by a
	// profile write, and absent until a sign-in has been recorded.
	LastLoginAt string `json:"lastLoginAt,omitempty" yaml:"-"`

	Kind providers.AgentKind `json:"kind,omitempty" yaml:"kind,omitempty"`

	inboundmodel.InboundAuthProfileReq `yaml:",inline"`
	InboundAuthConfig                  []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" yaml:"inboundAuthConfig,omitempty"`
}

// BasicAgentResponse is the summary view used in list responses.
type BasicAgentResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Display     string          `json:"display,omitempty"`
	Description string          `json:"description,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
	IsReadOnly  bool            `json:"isReadOnly"`
	// Status is the effective access status and what produced it, derived at read time.
	Status *providers.AccessStatusReport `json:"status,omitempty"`
	// LastLoginAt is when this agent was last granted access. Absent until a sign-in has been
	// recorded.
	LastLoginAt string `json:"lastLoginAt,omitempty"`

	Kind providers.AgentKind `json:"kind,omitempty"`
}

// SuspendOwnedAgentsResponse reports how many of the owner's agents a suspend-all suspended.
type SuspendOwnedAgentsResponse struct {
	SuspendedCount int `json:"suspendedCount"`
}

// UpdateOwnedAgentRequest is the body of an owner's update of their agent: the whole part the owner may change.
// Attributes, grant types and redirect URIs are required; a description left out is cleared.
type UpdateOwnedAgentRequest struct {
	Description  string                `json:"description,omitempty"`
	Attributes   json.RawMessage       `json:"attributes,omitempty"`
	GrantTypes   []providers.GrantType `json:"grantTypes,omitempty"`
	RedirectURIs []string              `json:"redirectUris,omitempty"`
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
