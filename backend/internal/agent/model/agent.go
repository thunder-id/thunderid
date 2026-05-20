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

// Package model defines the data transfer objects for the agent module.
package model

import (
	"encoding/json"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// CreateAgentRequest is the HTTP request body for creating an agent.
type CreateAgentRequest struct {
	OUID        string          `json:"ouId"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfile
	InboundAuthConfig []inboundmodel.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// UpdateAgentRequest is the HTTP request body for updating an agent.
type UpdateAgentRequest struct {
	OUID        string          `json:"ouId,omitempty"`
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfile
	InboundAuthConfig []inboundmodel.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
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
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfile
	InboundAuthConfig []inboundmodel.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
}

// AgentGetResponse is returned on read operations. Excludes secrets (no clientSecret).
type AgentGetResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`

	inboundmodel.InboundAuthProfile
	InboundAuthConfig []inboundmodel.InboundAuthConfig `json:"inboundAuthConfig,omitempty"`
}

// BasicAgentResponse is the summary view used in list responses.
type BasicAgentResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty"`
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	Owner       string          `json:"owner,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
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
