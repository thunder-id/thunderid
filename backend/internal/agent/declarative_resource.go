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

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

const (
	resourceTypeAgent = "agent"
	paramTypeAgent    = "Agent"
)

// agentExporter implements declarativeresource.ResourceExporter for agents.
type agentExporter struct {
	service AgentServiceInterface
}

func newAgentExporter(service AgentServiceInterface) *agentExporter {
	return &agentExporter{service: service}
}

// NewAgentExporterForTest exposes the exporter constructor for test packages.
func NewAgentExporterForTest(service AgentServiceInterface) *agentExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newAgentExporter(service)
}

// GetResourceType returns the resource type identifier for agents.
func (e *agentExporter) GetResourceType() string {
	return resourceTypeAgent
}

// GetParameterizerType returns the parameterizer type name for agents.
func (e *agentExporter) GetParameterizerType() string {
	return paramTypeAgent
}

// GetAllResourceIDs returns IDs of all mutable (non-declarative) agents.
// In composite mode declarative agents are excluded so they are not re-exported.
func (e *agentExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	ids := []string{}

	for {
		agents, err := e.service.GetAgentList(ctx, limit, offset, nil, false)
		if err != nil {
			return nil, err
		}

		for _, a := range agents.Agents {
			if !a.IsReadOnly {
				ids = append(ids, a.ID)
			}
		}

		offset += len(agents.Agents)
		if len(agents.Agents) == 0 {
			break
		}
	}

	return ids, nil
}

// GetResourceByID retrieves an agent by its ID for export.
func (e *agentExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *serviceerror.ServiceError) {
	a, err := e.service.GetAgent(ctx, id, false)
	if err != nil {
		return nil, "", err
	}
	// Decode Attributes from json.RawMessage into a map so the YAML marshaler can
	// serialize them correctly (json.RawMessage would encode as base64 in YAML).
	if len(a.Attributes) > 0 {
		var attrs map[string]interface{}
		if jsonErr := json.Unmarshal(a.Attributes, &attrs); jsonErr == nil {
			a.AttributesYAML = attrs
		}
	}
	return a, a.Name, nil
}

// ValidateResource validates an agent resource prior to export.
func (e *agentExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	a, ok := resource.(*model.AgentGetResponse)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeAgent, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		a.Name, resourceTypeAgent, id, "AGT_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return a.Name, nil
}

// GetResourceRules returns parameterization rules for agents with OAuth.
func (e *agentExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables: []string{
			"InboundAuthConfig[].OAuthConfig.ClientID",
			"InboundAuthConfig[].OAuthConfig.ClientSecret",
		},
		ArrayVariables: []string{
			"InboundAuthConfig[].OAuthConfig.RedirectURIs",
		},
	}
}

// GetResourceRulesForResource returns per-resource parameterization rules.
// Public clients omit ClientSecret. RedirectURIs are only parameterized when present
// (e.g. M2M agents using client_credentials have no redirect URIs).
func (e *agentExporter) GetResourceRulesForResource(resource interface{}) *declarativeresource.ResourceRules {
	a, ok := resource.(*model.AgentGetResponse)
	if !ok {
		return e.GetResourceRules()
	}

	isPublicClient := false
	hasRedirectURIs := false
	for _, inbound := range a.InboundAuthConfig {
		if inbound.OAuthConfig == nil {
			continue
		}
		if inbound.OAuthConfig.PublicClient {
			isPublicClient = true
		}
		if len(inbound.OAuthConfig.RedirectURIs) > 0 {
			hasRedirectURIs = true
		}
	}

	variables := []string{"InboundAuthConfig[].OAuthConfig.ClientID"}
	if !isPublicClient {
		variables = append(variables, "InboundAuthConfig[].OAuthConfig.ClientSecret")
	}

	rules := &declarativeresource.ResourceRules{Variables: variables}
	if hasRedirectURIs {
		rules.ArrayVariables = []string{"InboundAuthConfig[].OAuthConfig.RedirectURIs"}
	}
	return rules
}

// makeAgentDeclarativeConfig returns the entity loader configuration for agents.
func makeAgentDeclarativeConfig(agentSvc AgentServiceInterface) entity.DeclarativeLoaderConfig {
	return entity.DeclarativeLoaderConfig{
		Directory: "agents",
		Category:  entity.EntityCategoryAgent,
		Parser:    makeAgentEntityParser(agentSvc),
	}
}

// makeAgentEntityParser returns a parser that converts agent YAML into an entity.Entity.
func makeAgentEntityParser(
	agentSvc AgentServiceInterface,
) func([]byte) (*entity.Entity, json.RawMessage, json.RawMessage, error) {
	return func(data []byte) (*entity.Entity, json.RawMessage, json.RawMessage, error) {
		if agentSvc == nil {
			return nil, nil, nil, fmt.Errorf("agent service is required for declarative entity parsing")
		}

		var req model.AgentRequestWithID
		if err := yaml.Unmarshal(data, &req); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse agent YAML: %w", err)
		}

		var attributesJSON json.RawMessage
		if len(req.Attributes) > 0 {
			raw, err := json.Marshal(req.Attributes)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to marshal agent attributes: %w", err)
			}
			attributesJSON = raw
		}

		agent := &model.Agent{
			ID:                 req.ID,
			OUID:               req.OUID,
			OUHandle:           req.OUHandle,
			Type:               req.Type,
			Name:               req.Name,
			Description:        req.Description,
			Owner:              req.Owner,
			Attributes:         attributesJSON,
			InboundAuthProfile: req.InboundAuthProfile,
			InboundAuthConfig:  req.InboundAuthConfig,
		}
		clientID, clientSecret, _, svcErr := agentSvc.ValidateAgent(
			security.WithRuntimeContext(context.Background()), agent, req.ID)
		if svcErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to validate agent '%s': %v", req.ID, svcErr)
		}

		// agent.OUID may have been resolved from OUHandle by ValidateAgent.
		e, sysCredsJSON, buildErr := buildAgentEntity(
			req.ID, req.Type, agent.OUID, attributesJSON,
			req.Name, req.Description, req.Owner, clientID, clientSecret,
		)
		if buildErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to build agent entity for '%s': %w", req.ID, buildErr)
		}

		return e, nil, sysCredsJSON, nil
	}
}

// makeAgentInboundConfig returns the inbound client loader configuration for agents.
func makeAgentInboundConfig(agentSvc AgentServiceInterface) inboundmodel.DeclarativeLoaderConfig {
	return inboundmodel.DeclarativeLoaderConfig{
		ResourceType:  "Agent",
		DirectoryName: "agents",
		Parser:        makeAgentInboundParser(agentSvc),
		Validator: func(p *inboundmodel.InboundClient) error {
			if p == nil {
				return fmt.Errorf("parsed inbound client is nil")
			}
			return nil
		},
	}
}

// makeAgentInboundParser returns a parser that converts agent YAML into an InboundClient.
func makeAgentInboundParser(agentSvc AgentServiceInterface) func([]byte) (*inboundmodel.InboundClient, error) {
	return func(data []byte) (*inboundmodel.InboundClient, error) {
		var req model.AgentRequestWithID
		if err := yaml.Unmarshal(data, &req); err != nil {
			return nil, fmt.Errorf("failed to parse agent YAML: %w", err)
		}

		agent := &model.Agent{
			ID:                 req.ID,
			OUID:               req.OUID,
			OUHandle:           req.OUHandle,
			Type:               req.Type,
			Name:               req.Name,
			Description:        req.Description,
			Owner:              req.Owner,
			InboundAuthProfile: req.InboundAuthProfile,
			InboundAuthConfig:  req.InboundAuthConfig,
		}
		_, _, resolvedClient, svcErr := agentSvc.ValidateAgent(
			security.WithRuntimeContext(context.Background()), agent, req.ID)
		if svcErr != nil {
			return nil, fmt.Errorf("failed to validate agent '%s': %v", req.ID, svcErr)
		}

		resolvedClient.ID = req.ID
		resolvedClient.IsReadOnly = true

		oauthProfile := buildOAuthProfile(req.InboundAuthConfig)
		if oauthProfile != nil {
			if resolvedClient.Properties == nil {
				resolvedClient.Properties = make(map[string]interface{})
			}
			resolvedClient.Properties[inboundclient.PropOAuthProfile] = *oauthProfile
		}

		return &resolvedClient, nil
	}
}
