// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agenttype

import (
	"encoding/json"
)

// AgentType represents an agent type as returned by the detail endpoint.
type AgentType struct {
	ID          string          `json:"id,omitempty"`
	Handle      string          `json:"handle,omitempty"`
	DisplayName string          `json:"displayName,omitempty"`
	OUID        string          `json:"ouId"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

// AgentTypeRequest is the body for creating or updating an agent type.
type AgentTypeRequest struct {
	Handle      string          `json:"handle,omitempty"`
	DisplayName string          `json:"displayName"`
	OUID        string          `json:"ouId"`
	Schema      json.RawMessage `json:"schema"`
}

// AgentTypeListItem is a list entry. The list endpoint omits the schema.
type AgentTypeListItem struct {
	ID          string `json:"id,omitempty"`
	Handle      string `json:"handle,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	OUID        string `json:"ouId"`
}

// AgentTypeListResponse is the response from listing agent types. The entries live under `types`.
type AgentTypeListResponse struct {
	TotalResults int                 `json:"totalResults"`
	StartIndex   int                 `json:"startIndex"`
	Count        int                 `json:"count"`
	Types        []AgentTypeListItem `json:"types"`
}

// I18nMessage is an API message, which the server emits as an object but older surfaces emit as a
// plain string.
type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// UnmarshalJSON accepts either a bare string or the structured form.
func (m *I18nMessage) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.DefaultValue = s
		return nil
	}
	type alias I18nMessage
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = I18nMessage(a)
	return nil
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description,omitempty"`
}

// Agent is the subset of the agent model this suite needs to exercise schema-driven uniqueness.
type Agent struct {
	ID         string          `json:"id,omitempty"`
	OUID       string          `json:"ouId,omitempty"`
	Type       string          `json:"type,omitempty"`
	Name       string          `json:"name,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// AgentListResponse is the response from listing agents.
type AgentListResponse struct {
	TotalResults int     `json:"totalResults"`
	Count        int     `json:"count"`
	Agents       []Agent `json:"agents"`
}
