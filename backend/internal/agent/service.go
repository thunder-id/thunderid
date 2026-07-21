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

// Package agent provides functionality for managing agents
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// AgentServiceInterface defines the operations exposed by the agent service.
type AgentServiceInterface interface {
	CreateAgent(ctx context.Context, agent *model.Agent) (*model.AgentCompleteResponse,
		*tidcommon.ServiceError)
	GetAgent(ctx context.Context, agentID string, includeDisplay bool) (*model.AgentGetResponse,
		*tidcommon.ServiceError)
	GetAgentList(ctx context.Context, limit, offset int, filters map[string]interface{},
		includeDisplay bool) (*model.AgentListResponse, *tidcommon.ServiceError)
	UpdateAgent(ctx context.Context, agentID string, req *model.UpdateAgentRequest) (
		*model.AgentCompleteResponse, *tidcommon.ServiceError)
	DeleteAgent(ctx context.Context, agentID string) *tidcommon.ServiceError
	GetAgentGroups(ctx context.Context, agentID string, limit, offset int) (
		*model.AgentGroupListResponse, *tidcommon.ServiceError)
	GetAgentRoles(ctx context.Context, agentID string, limit, offset int) (
		*model.AgentRoleListResponse, *tidcommon.ServiceError)
	ValidateAgent(ctx context.Context, agent *model.Agent, excludeID string) (
		clientID, clientSecret string, client inboundmodel.InboundClient, svcErr *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	SetDependencyRegistry(r resourcedependency.Registry)
}

type agentService struct {
	logger               *log.Logger
	entityService        entity.EntityServiceInterface
	inboundClientService inboundclient.InboundClientServiceInterface
	ouService            oupkg.OrganizationUnitServiceInterface
	dependencyRegistry   resourcedependency.Registry
	roleService          role.RoleServiceInterface
}

func newAgentService(
	entityService entity.EntityServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	roleService role.RoleServiceInterface,
) AgentServiceInterface {
	return &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		entityService:        entityService,
		inboundClientService: inboundClientService,
		ouService:            ouService,
		roleService:          roleService,
	}
}

// CreateAgent creates an agent entity with optional inbound auth profile.
func (s *agentService) CreateAgent(ctx context.Context, agent *model.Agent) (
	*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if agent == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	normalizeLoginConsent(agent.LoginConsent)

	clientID, clientSecret, _, svcErr := s.ValidateAgent(ctx, agent, "")
	if svcErr != nil {
		return nil, svcErr
	}

	agentID := agent.ID
	if agentID == "" {
		var err error
		agentID, err = sysutils.GenerateUUIDv7()
		if err != nil {
			s.logger.Error(ctx, "Failed to generate agent ID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
	}

	owner := agent.Owner
	if owner == "" {
		owner = security.GetSubject(ctx)
	} else if svcErr := s.validateOwnerExists(ctx, owner); svcErr != nil {
		return nil, svcErr
	}

	e, sysCredsJSON, buildErr := buildAgentEntity(agentID, agent.Type, agent.OUID, agent.Attributes,
		agent.Name, agent.Description, owner, clientID, clientSecret)
	if buildErr != nil {
		s.logger.Error(ctx, "Failed to build agent entity", log.Error(buildErr))
		return nil, &tidcommon.InternalServerError
	}

	createdEntity, entErr := s.entityService.CreateEntity(ctx, e, sysCredsJSON)
	if entErr != nil {
		if mapped := mapEntityError(entErr); mapped != nil {
			return nil, mapped
		}
		s.logger.Error(ctx, "Failed to create agent entity",
			log.String("agentID", agentID), log.Error(entErr))
		return nil, &tidcommon.InternalServerError
	}

	authFlowID, regFlowID := agent.AuthFlowID, agent.RegistrationFlowID
	assertion, loginConsent := agent.Assertion, agent.LoginConsent
	var inboundConfigs []providers.InboundAuthConfigWithSecret

	if needsInboundClient(agent) {
		resolvedClient, resolvedOAuth, svcErr := s.createInboundForAgent(ctx, agentID, agent, clientSecret)
		if svcErr != nil {
			s.deleteEntityCompensation(ctx, agentID)
			return nil, svcErr
		}
		authFlowID = resolvedClient.AuthFlowID
		regFlowID = resolvedClient.RegistrationFlowID
		assertion = resolvedClient.Assertion
		loginConsent = resolvedClient.LoginConsent
		if resolvedOAuth != nil {
			inboundConfigs = []providers.InboundAuthConfigWithSecret{{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: oauthProfileToComplete(clientID, resolvedOAuth),
			}}
		}
	}

	resp := buildCompleteResponse(agentID, owner, clientID, clientSecret,
		agent.Type, agent.Name, agent.Description, createdEntity.Attributes,
		authFlowID, regFlowID, agent.IsRegistrationFlowEnabled,
		agent.ThemeID, agent.LayoutID, assertion, loginConsent,
		agent.AllowedUserTypes, inboundConfigs)
	resp.OUID = agent.OUID
	s.populateOUHandleForComplete(ctx, resp)
	return resp, nil
}

// GetAgent returns a single agent by ID.
func (s *agentService) GetAgent(ctx context.Context, agentID string, includeDisplay bool) (
	*model.AgentGetResponse, *tidcommon.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}

	e, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent entity",
			log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if e.Category != providers.EntityCategoryAgent {
		return nil, &ErrorAgentNotFound
	}

	resp, svcErr := s.composeGetResponse(ctx, e)
	if svcErr != nil {
		return nil, svcErr
	}

	if includeDisplay {
		s.populateOUHandleForGet(ctx, resp)
	}

	return resp, nil
}

// GetAgentList returns a paginated list of agents.
func (s *agentService) GetAgentList(ctx context.Context, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (
	*model.AgentListResponse, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = 30
	}

	totalCount, err := s.entityService.GetEntityListCount(ctx, providers.EntityCategoryAgent, filters)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent list count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	entities, err := s.entityService.GetEntityList(ctx, providers.EntityCategoryAgent, limit, offset, filters)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent list", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return s.buildListResponse(ctx, entities, totalCount, limit, offset, includeDisplay), nil
}

// UpdateAgent applies a full-replacement update to the agent.
func (s *agentService) UpdateAgent(ctx context.Context, agentID string,
	req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}
	if req == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if svcErr := validateBaseFields(req.Name, req.Type); svcErr != nil {
		return nil, svcErr
	}
	existing, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent entity for update",
			log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing.Category != providers.EntityCategoryAgent {
		return nil, &ErrorAgentNotFound
	}
	if existing.IsReadOnly {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	currentName, _, currentOwner, currentClientID := readSystemAttributes(existing.SystemAttributes)
	if req.Name != currentName {
		if svcErr := s.validateNameUnique(ctx, req.Name, agentID); svcErr != nil {
			return nil, svcErr
		}
	}

	normalizeLoginConsent(req.LoginConsent)

	var existingOAuthMethod string
	existingOAuth, oauthErr := s.inboundClientService.GetOAuthProfileByEntityID(ctx, agentID)
	if oauthErr != nil && !errors.Is(oauthErr, inboundclient.ErrInboundClientNotFound) {
		s.logger.Error(ctx, "Failed to load existing OAuth profile",
			log.String("agentID", agentID), log.Error(oauthErr))
		return nil, &tidcommon.InternalServerError
	}
	if existingOAuth != nil {
		existingOAuthMethod = existingOAuth.TokenEndpointAuthMethod
	}

	clientID, clientSecret, svcErr := s.resolveOAuthCredentials(
		ctx, req.InboundAuthConfig, currentClientID, existingOAuthMethod)
	if svcErr != nil {
		return nil, svcErr
	}

	owner, svcErr := s.resolveUpdateOwner(ctx, req.Owner, currentOwner)
	if svcErr != nil {
		return nil, svcErr
	}

	ouID, svcErr := s.resolveUpdateOUID(ctx, req, existing.OUID)
	if svcErr != nil {
		return nil, svcErr
	}

	resolvedClient, resolvedOAuth, svcErr := s.reconcileInboundForUpdate(
		ctx, agentID, req, clientID, clientSecret)
	if svcErr != nil {
		return nil, svcErr
	}

	if resolvedOAuth == nil {
		clientID = ""
		clientSecret = ""
	}

	updatedEntity := &providers.Entity{
		ID:         agentID,
		Category:   providers.EntityCategoryAgent,
		Type:       req.Type,
		State:      providers.EntityStateActive,
		OUID:       ouID,
		Attributes: req.Attributes,
	}
	sysAttrsJSON, marshalErr := buildSystemAttributesJSON(req.Name, req.Description, owner, clientID)
	if marshalErr != nil {
		s.logger.Error(ctx, "Failed to build system attributes for update", log.Error(marshalErr))
		return nil, &tidcommon.InternalServerError
	}
	updatedEntity.SystemAttributes = sysAttrsJSON

	if _, err := s.entityService.UpdateEntity(ctx, agentID, updatedEntity); err != nil {
		if mapped := mapEntityError(err); mapped != nil {
			return nil, mapped
		}
		s.logger.Error(ctx, "Failed to update agent entity", log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if clientSecret != "" {
		sysCredsJSON, credErr := buildSystemCredentialsJSON(clientSecret)
		if credErr == nil && sysCredsJSON != nil {
			if err := s.entityService.UpdateSystemCredentials(ctx, agentID, sysCredsJSON); err != nil {
				s.logger.Error(ctx, "Failed to update agent system credentials",
					log.String("agentID", agentID), log.Error(err))
				return nil, &tidcommon.InternalServerError
			}
		}
	}

	authFlowID := resolvedClient.AuthFlowID
	regFlowID := resolvedClient.RegistrationFlowID
	assertion := resolvedClient.Assertion
	loginConsent := resolvedClient.LoginConsent
	var inboundConfigs []providers.InboundAuthConfigWithSecret
	if resolvedOAuth != nil {
		inboundConfigs = []providers.InboundAuthConfigWithSecret{{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: oauthProfileToComplete(clientID, resolvedOAuth),
		}}
	}

	resp := buildCompleteResponse(agentID, owner, clientID, clientSecret,
		req.Type, req.Name, req.Description, req.Attributes,
		authFlowID, regFlowID, resolvedClient.IsRegistrationFlowEnabled,
		req.ThemeID, req.LayoutID, assertion, loginConsent,
		req.AllowedUserTypes, inboundConfigs)
	resp.OUID = ouID
	s.populateOUHandleForComplete(ctx, resp)
	return resp, nil
}

// DeleteAgent removes the agent and its associated inbound client.
// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (s *agentService) SetDependencyRegistry(r resourcedependency.Registry) {
	s.dependencyRegistry = r
}

func (s *agentService) DeleteAgent(ctx context.Context, agentID string) *tidcommon.ServiceError {
	if agentID == "" {
		return &ErrorMissingAgentID
	}

	existing, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent for delete",
			log.String("agentID", agentID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if existing.Category != providers.EntityCategoryAgent {
		return &ErrorAgentNotFound
	}
	if existing.IsReadOnly {
		return &ErrorCannotModifyDeclarativeResource
	}

	// Remove dependents that must be deleted with the agent (e.g. its role assignments and group
	// memberships). Run before the deletes so a cleanup failure aborts and leaves the agent
	// retriable. Fails closed when the registry is unavailable.
	if s.dependencyRegistry == nil {
		s.logger.Error(ctx, "Dependency registry not set; refusing to delete agent",
			log.String("agentID", agentID))
		return &tidcommon.InternalServerError
	}
	if _, err := s.dependencyRegistry.CascadeDelete(ctx, resourcedependency.ResourceTypeAgent, agentID); err != nil {
		s.logger.Error(ctx, "Failed to cascade-delete agent dependencies",
			log.String("agentID", agentID), log.Error(err))
		return &tidcommon.InternalServerError
	}

	if err := s.inboundClientService.DeleteInboundClient(ctx, agentID); err != nil &&
		!errors.Is(err, inboundclient.ErrInboundClientNotFound) {
		if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
			return svcErr
		}
		s.logger.Error(ctx, "Failed to delete inbound client for agent",
			log.Error(err), log.String("agentID", agentID))
		return &tidcommon.InternalServerError
	}

	if err := s.entityService.DeleteEntity(ctx, agentID); err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil
		}
		s.logger.Error(ctx, "Failed to delete agent entity", log.String("agentID", agentID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// GetResourceDependencies returns the agents that reference the resource identified by
// (resourceType, id). It implements the resourcedependency.Provider interface.
//
// Agents reference a user through their owner attribute, which lives on the agent entity rather
// than in the inbound-client store, so user dependencies are resolved separately. All other
// reference types are resolved via the inbound-client store, which decides what is tracked. The
// number of referencing entities is bounded by MaxCompositeStoreRecords.
func (s *agentService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType == resourcedependency.ResourceTypeUser {
		return s.getAgentsByOwner(ctx, id)
	}

	ids, _, err := s.inboundClientService.GetEntityIDsByReference(
		ctx, resourceType, id, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		s.logger.Error(ctx, "Failed to get entity IDs by reference", log.Error(err))
		return nil, err
	}
	if len(ids) == 0 {
		return []resourcedependency.ResourceDependency{}, nil
	}

	entities, err := s.entityService.GetEntitiesByIDs(ctx, ids)
	if err != nil {
		s.logger.Error(ctx, "Failed to get entities by IDs", log.Error(err))
		return nil, err
	}

	usages := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, e := range entities {
		// Applications and agents share the inbound-client store; only report agents.
		if e.Category != providers.EntityCategoryAgent {
			continue
		}
		name, _, _, _ := readSystemAttributes(e.SystemAttributes)
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeAgent,
			ID:               e.ID,
			DisplayName:      name,
			BehaviorOnDelete: resourcedependency.BehaviorFallback,
		})
	}
	return usages, nil
}

// getAgentsByOwner returns the agents that list the given user as their owner. The owner is stored
// in the agent entity's system attributes, which the entity list filter does not search (it only
// matches the public attributes column), so agents are listed and matched on owner in memory. The
// number of agents scanned is bounded by MaxCompositeStoreRecords.
func (s *agentService) getAgentsByOwner(
	ctx context.Context, ownerID string) ([]resourcedependency.ResourceDependency, error) {
	entities, err := s.entityService.GetEntityList(
		ctx, providers.EntityCategoryAgent, serverconst.MaxCompositeStoreRecords, 0, nil)
	if err != nil {
		s.logger.Error(ctx, "Failed to list agents", log.Error(err))
		return nil, err
	}

	usages := make([]resourcedependency.ResourceDependency, 0)
	for _, e := range entities {
		if e.Category != providers.EntityCategoryAgent {
			continue
		}
		name, _, owner, _ := readSystemAttributes(e.SystemAttributes)
		if owner != ownerID {
			continue
		}
		// An agent cannot exist without its owner, so ownership blocks the owner's deletion.
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeAgent,
			ID:               e.ID,
			DisplayName:      name,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return usages, nil
}

// GetAgentGroups returns the groups the agent belongs to.
func (s *agentService) GetAgentGroups(ctx context.Context, agentID string, limit, offset int) (
	*model.AgentGroupListResponse, *tidcommon.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = 30
	}

	existing, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent for groups",
			log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing.Category != providers.EntityCategoryAgent {
		return nil, &ErrorAgentNotFound
	}

	totalCount, err := s.entityService.GetGroupCountForEntity(ctx, agentID)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent group count",
			log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	groups, err := s.entityService.GetEntityGroups(ctx, agentID, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent groups", log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	out := make([]model.AgentGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, model.AgentGroup{ID: g.ID, Name: g.Name, OUID: g.OUID})
	}

	resp := &model.AgentGroupListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(out),
		Groups:       out,
		Links: sysutils.BuildPaginationLinks(
			fmt.Sprintf("%s/%s/groups", agentBasePath, agentID), limit, offset, totalCount, ""),
	}
	return resp, nil
}

// GetAgentRoles returns the roles assigned to the agent, either directly or through its
// group memberships.
func (s *agentService) GetAgentRoles(ctx context.Context, agentID string, limit, offset int) (
	*model.AgentRoleListResponse, *tidcommon.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = 30
	}

	existing, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent for roles",
			log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing.Category != providers.EntityCategoryAgent {
		return nil, &ErrorAgentNotFound
	}

	groupCount, err := s.entityService.GetGroupCountForEntity(ctx, agentID)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent group count",
			log.String("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	groupIDs := make([]string, 0, groupCount)
	if groupCount > 0 {
		groups, groupErr := s.entityService.GetEntityGroups(ctx, agentID, groupCount, 0)
		if groupErr != nil {
			s.logger.Error(ctx, "Failed to get agent groups for role lookup",
				log.String("agentID", agentID), log.Error(groupErr))
			return nil, &tidcommon.InternalServerError
		}
		for _, g := range groups {
			groupIDs = append(groupIDs, g.ID)
		}
	}

	roles, svcErr := s.roleService.GetUserRoles(ctx, agentID, groupIDs)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to get agent roles", log.String("agentID", agentID))
		return nil, svcErr
	}

	totalCount := len(roles)
	end := offset + limit
	if offset > totalCount {
		offset = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	page := roles[offset:end]

	resp := &model.AgentRoleListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(page),
		Roles:        page,
		Links: sysutils.BuildPaginationLinks(
			fmt.Sprintf("%s/%s/roles", agentBasePath, agentID), limit, offset, totalCount, ""),
	}
	return resp, nil
}

// ValidateAgent validates an Agent without persisting. It resolves OAuth credentials
// using the entity ID (excludeID) for exclusion, allowing declarative reload of an existing agent.
func (s *agentService) ValidateAgent(ctx context.Context, agent *model.Agent, excludeID string) (
	string, string, inboundmodel.InboundClient, *tidcommon.ServiceError) {
	if agent == nil {
		return "", "", inboundmodel.InboundClient{}, &ErrorInvalidRequestFormat
	}
	if svcErr := validateBaseFields(agent.Name, agent.Type); svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	if agent.OUID == "" && agent.OUHandle != "" {
		ou, svcErr := s.ouService.GetOrganizationUnitByPath(ctx, agent.OUHandle)
		if svcErr != nil {
			if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				return "", "", inboundmodel.InboundClient{}, &ErrorOrganizationUnitNotFound
			}
			s.logger.Error(ctx, "Failed to resolve OU handle", log.Any("error", svcErr))
			return "", "", inboundmodel.InboundClient{}, &tidcommon.InternalServerError
		}
		agent.OUID = ou.ID
	}
	if svcErr := s.validateOUExists(ctx, agent.OUID); svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	if svcErr := s.validateNameUnique(ctx, agent.Name, excludeID); svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}

	var clientID, clientSecret string
	oauthCfg, svcErr := pickOAuthConfig(agent.InboundAuthConfig)
	if svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	if oauthCfg != nil {
		clientID = oauthCfg.ClientID
		if clientID == "" {
			generated, err := oauthutils.GenerateOAuth2ClientID()
			if err != nil {
				s.logger.Error(ctx, "Failed to generate client ID", log.Error(err))
				return "", "", inboundmodel.InboundClient{}, &tidcommon.InternalServerError
			}
			clientID = generated
		} else if taken, checkErr := s.isClientIDTaken(ctx, clientID, excludeID); checkErr != nil {
			return "", "", inboundmodel.InboundClient{}, checkErr
		} else if taken {
			return "", "", inboundmodel.InboundClient{}, &ErrorAgentAlreadyExistsWithClientID
		}

		if requiresClientSecret(oauthCfg) && oauthCfg.ClientSecret == "" {
			generated, err := oauthutils.GenerateOAuth2ClientSecret()
			if err != nil {
				s.logger.Error(ctx, "Failed to generate client secret", log.Error(err))
				return "", "", inboundmodel.InboundClient{}, &tidcommon.InternalServerError
			}
			clientSecret = generated
		} else {
			clientSecret = oauthCfg.ClientSecret
		}
	}

	if err := s.inboundClientService.ResolveInboundAuthProfileHandles(ctx, &agent.InboundAuthProfile); err != nil {
		if svcErr := translateInboundClientFKError(err); svcErr != nil {
			return "", "", inboundmodel.InboundClient{}, svcErr
		}
		s.logger.Error(ctx, "Failed to resolve inbound auth profile handles", log.Error(err))
		return "", "", inboundmodel.InboundClient{}, &tidcommon.InternalServerError
	}

	client := buildInboundClientRecord("", agent.AuthFlowID, agent.RegistrationFlowID,
		agent.IsRegistrationFlowEnabled, agent.ThemeID, agent.LayoutID, agent.Assertion,
		agent.LoginConsent, agent.AllowedUserTypes)

	if needsInboundClient(agent) {
		oauthProfile := buildOAuthProfile(agent.InboundAuthConfig)
		hasSecret := clientSecret != ""
		if err := s.inboundClientService.Validate(ctx, &client, oauthProfile, hasSecret); err != nil {
			if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
				return "", "", inboundmodel.InboundClient{}, svcErr
			}
			s.logger.Error(ctx, "Inbound client validation failed", log.Error(err))
			return "", "", inboundmodel.InboundClient{}, &tidcommon.InternalServerError
		}
	}

	return clientID, clientSecret, client, nil
}

// deleteEntityCompensation deletes the entity row as a best-effort rollback after a failed downstream operation.
func (s *agentService) deleteEntityCompensation(ctx context.Context, agentID string) {
	if err := s.entityService.DeleteEntity(ctx, agentID); err != nil {
		s.logger.Error(ctx, "Failed to delete entity during compensation",
			log.String("agentID", agentID), log.Error(err))
	}
}

// validateOUExists returns an error if the given OU is empty or does not exist.
func (s *agentService) validateOUExists(ctx context.Context, ouID string) *tidcommon.ServiceError {
	if ouID == "" {
		return &ErrorOrganizationUnitNotFound
	}
	exists, err := s.ouService.IsOrganizationUnitExists(ctx, ouID)
	if err != nil {
		if err.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return &ErrorOrganizationUnitNotFound
		}
		s.logger.Error(ctx, "Failed to verify OU existence", log.String("ouID", ouID), log.Any("error", err))
		return &tidcommon.InternalServerError
	}
	if !exists {
		return &ErrorOrganizationUnitNotFound
	}
	return nil
}

// resolveUpdateOwner picks the effective owner for an update — either the requested owner or the
// existing one — and validates it exists when the owner is changing.
func (s *agentService) resolveUpdateOUID(
	ctx context.Context, req *model.UpdateAgentRequest, existingOUID string,
) (string, *tidcommon.ServiceError) {
	if req.OUID == "" && req.OUHandle != "" {
		ou, ouSvcErr := s.ouService.GetOrganizationUnitByPath(ctx, req.OUHandle)
		if ouSvcErr != nil {
			if ouSvcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				return "", &ErrorOrganizationUnitNotFound
			}
			s.logger.Error(ctx, "Failed to resolve OU handle", log.Any("error", ouSvcErr))
			return "", &tidcommon.InternalServerError
		}
		req.OUID = ou.ID
	}
	ouID := req.OUID
	if ouID == "" {
		return existingOUID, nil
	}
	if ouID != existingOUID {
		if svcErr := s.validateOUExists(ctx, ouID); svcErr != nil {
			return "", svcErr
		}
	}
	return ouID, nil
}

func (s *agentService) resolveUpdateOwner(
	ctx context.Context, requestedOwner, currentOwner string,
) (string, *tidcommon.ServiceError) {
	owner := requestedOwner
	if owner == "" {
		owner = currentOwner
	}
	if owner != currentOwner {
		if svcErr := s.validateOwnerExists(ctx, owner); svcErr != nil {
			return "", svcErr
		}
	}
	return owner, nil
}

// validateOwnerExists returns an error when the given owner ID does not resolve to an existing entity.
func (s *agentService) validateOwnerExists(ctx context.Context, ownerID string) *tidcommon.ServiceError {
	if ownerID == "" {
		return nil
	}
	_, err := s.entityService.GetEntity(ctx, ownerID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorOwnerNotFound
		}
		s.logger.Error(ctx, "Failed to verify owner existence",
			log.String("ownerID", ownerID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// validateNameUnique returns an error if another agent already uses the given name (excludeID is exempt on updates).
func (s *agentService) validateNameUnique(ctx context.Context, name, excludeID string) *tidcommon.ServiceError {
	if name == "" {
		return &ErrorInvalidAgentName
	}
	id, err := s.entityService.IdentifyEntity(ctx, map[string]interface{}{fieldName: name})
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil
		}
		if errors.Is(err, entity.ErrAmbiguousEntity) {
			return &ErrorAgentAlreadyExistsWithName
		}
		s.logger.Error(ctx, "Failed to verify agent name uniqueness", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if id == nil || *id == "" {
		return nil
	}
	if excludeID != "" && *id == excludeID {
		return nil
	}
	// Verify the found entity is actually an agent before treating it as a name conflict.
	// IdentifyEntity searches across all entity categories; apps also store their name in
	// system attributes under the same key.
	found, getErr := s.entityService.GetEntity(ctx, *id)
	if getErr != nil || found.Category != providers.EntityCategoryAgent {
		return nil
	}
	return &ErrorAgentAlreadyExistsWithName
}

// resolveOAuthCredentials resolves the clientID and clientSecret for an agent OAuth profile.
func (s *agentService) resolveOAuthCredentials(ctx context.Context,
	configs []providers.InboundAuthConfigWithSecret, existingClientID, existingOAuthMethod string,
) (string, string, *tidcommon.ServiceError) {
	oauthCfg, svcErr := pickOAuthConfig(configs)
	if svcErr != nil {
		return "", "", svcErr
	}
	if oauthCfg == nil {
		return existingClientID, "", nil
	}

	clientID := oauthCfg.ClientID
	if clientID == "" {
		clientID = existingClientID
	}
	if clientID == "" {
		generated, err := oauthutils.GenerateOAuth2ClientID()
		if err != nil {
			s.logger.Error(ctx, "Failed to generate client ID", log.Error(err))
			return "", "", &tidcommon.InternalServerError
		}
		clientID = generated
	} else if clientID != existingClientID {
		taken, svcErr := s.isClientIDTaken(ctx, clientID, existingClientID)
		if svcErr != nil {
			return "", "", svcErr
		}
		if taken {
			return "", "", &ErrorAgentAlreadyExistsWithClientID
		}
	}

	clientSecret := oauthCfg.ClientSecret
	requiresSecret := requiresClientSecret(oauthCfg)
	if requiresSecret && clientSecret == "" {
		existingWasSecretBased := existingOAuthMethod == string(providers.TokenEndpointAuthMethodClientSecretBasic) ||
			existingOAuthMethod == string(providers.TokenEndpointAuthMethodClientSecretPost)
		if !existingWasSecretBased {
			generated, err := oauthutils.GenerateOAuth2ClientSecret()
			if err != nil {
				s.logger.Error(ctx, "Failed to generate client secret", log.Error(err))
				return "", "", &tidcommon.InternalServerError
			}
			clientSecret = generated
		}
	}
	return clientID, clientSecret, nil
}

// isClientIDTaken reports whether the given clientID is already used by a different entity.
func (s *agentService) isClientIDTaken(
	ctx context.Context, clientID, excludeID string) (bool, *tidcommon.ServiceError) {
	id, err := s.entityService.IdentifyEntity(ctx, map[string]interface{}{fieldClientID: clientID})
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return false, nil
		}
		s.logger.Error(ctx, "Failed to check client ID availability", log.MaskedString("clientID", clientID),
			log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	if id == nil || *id == "" {
		return false, nil
	}
	if excludeID != "" && *id == excludeID {
		return false, nil
	}
	return true, nil
}

// createInboundForAgent creates the inbound client row; applies server defaults via CreateInboundClient.
func (s *agentService) createInboundForAgent(ctx context.Context, agentID string,
	agent *model.Agent, clientSecret string) (
	inboundmodel.InboundClient, *providers.OAuthProfile, *tidcommon.ServiceError) {
	client := buildInboundClientRecord(agentID, agent.AuthFlowID, agent.RegistrationFlowID,
		agent.IsRegistrationFlowEnabled, agent.ThemeID, agent.LayoutID, agent.Assertion,
		agent.LoginConsent, agent.AllowedUserTypes)

	oauthProfile := buildOAuthProfile(agent.InboundAuthConfig)

	hasSecret := clientSecret != ""
	if err := s.inboundClientService.CreateInboundClient(
		ctx, &client, oauthProfile, hasSecret,
	); err != nil {
		if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
			return inboundmodel.InboundClient{}, nil, svcErr
		}
		s.logger.Error(ctx, "Failed to create inbound client for agent",
			log.Error(err), log.String("agentID", agentID))
		return inboundmodel.InboundClient{}, nil, &tidcommon.InternalServerError
	}
	return client, oauthProfile, nil
}

// reconcileInboundForUpdate creates, updates, or removes the inbound client row and returns the mutated structs.
func (s *agentService) reconcileInboundForUpdate(ctx context.Context, agentID string,
	req *model.UpdateAgentRequest, clientID, clientSecret string,
) (inboundmodel.InboundClient, *providers.OAuthProfile, *tidcommon.ServiceError) {
	wantsInbound := updateNeedsInboundClient(req)

	existingClient, getErr := s.inboundClientService.GetInboundClientByEntityID(ctx, agentID)
	hasExisting := getErr == nil && existingClient != nil

	if !wantsInbound {
		if hasExisting {
			if err := s.inboundClientService.DeleteInboundClient(ctx, agentID); err != nil &&
				!errors.Is(err, inboundclient.ErrInboundClientNotFound) {
				if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
					return inboundmodel.InboundClient{}, nil, svcErr
				}
				s.logger.Error(ctx, "Failed to remove inbound client during update",
					log.Error(err), log.String("agentID", agentID))
				return inboundmodel.InboundClient{}, nil, &tidcommon.InternalServerError
			}
		}
		return inboundmodel.InboundClient{}, nil, nil
	}

	client := buildInboundClientRecord(agentID, req.AuthFlowID, req.RegistrationFlowID,
		req.IsRegistrationFlowEnabled, req.ThemeID, req.LayoutID, req.Assertion,
		req.LoginConsent, req.AllowedUserTypes)
	oauthProfile := buildOAuthProfile(req.InboundAuthConfig)
	hasSecret := clientSecret != ""

	if hasExisting {
		if err := s.inboundClientService.UpdateInboundClient(ctx, &client,
			oauthProfile, hasSecret, clientID); err != nil {
			if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
				return inboundmodel.InboundClient{}, nil, svcErr
			}
			s.logger.Error(ctx, "Failed to update inbound client",
				log.Error(err), log.String("agentID", agentID))
			return inboundmodel.InboundClient{}, nil, &tidcommon.InternalServerError
		}
		return client, oauthProfile, nil
	}

	if err := s.inboundClientService.CreateInboundClient(ctx, &client, oauthProfile, hasSecret); err != nil {
		if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
			return inboundmodel.InboundClient{}, nil, svcErr
		}
		s.logger.Error(ctx, "Failed to create inbound client during update",
			log.Error(err), log.String("agentID", agentID))
		return inboundmodel.InboundClient{}, nil, &tidcommon.InternalServerError
	}
	return client, oauthProfile, nil
}

// composeGetResponse builds the GET response by loading inbound client, OAuth profile, and certificates for the entity.
func (s *agentService) composeGetResponse(ctx context.Context, e *providers.Entity) (
	*model.AgentGetResponse, *tidcommon.ServiceError) {
	name, description, owner, clientID := readSystemAttributes(e.SystemAttributes)

	resp := &model.AgentGetResponse{
		ID:          e.ID,
		OUID:        e.OUID,
		OUHandle:    e.OUHandle,
		Type:        e.Type,
		Name:        name,
		Description: description,
		Owner:       owner,
		ClientID:    clientID,
		Attributes:  e.Attributes,
	}

	inbound, err := s.inboundClientService.GetInboundClientByEntityID(ctx, e.ID)
	if err != nil {
		if !errors.Is(err, inboundclient.ErrInboundClientNotFound) {
			s.logger.Error(ctx, "Failed to load inbound client for agent",
				log.String("agentID", e.ID), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		return resp, nil
	}

	resp.AuthFlowID = inbound.AuthFlowID
	resp.RegistrationFlowID = inbound.RegistrationFlowID
	resp.IsRegistrationFlowEnabled = inbound.IsRegistrationFlowEnabled
	resp.ThemeID = inbound.ThemeID
	resp.LayoutID = inbound.LayoutID
	resp.Assertion = inbound.Assertion
	resp.LoginConsent = inbound.LoginConsent
	resp.AllowedUserTypes = inbound.AllowedUserTypes

	oauth, oauthErr := s.inboundClientService.GetOAuthProfileByEntityID(ctx, e.ID)
	if oauthErr != nil && !errors.Is(oauthErr, inboundclient.ErrInboundClientNotFound) {
		s.logger.Error(ctx, "Failed to load OAuth profile for agent",
			log.String("agentID", e.ID), log.Error(oauthErr))
		return nil, &tidcommon.InternalServerError
	}
	if oauthErr == nil && oauth != nil {
		resp.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{
			{
				Type:        providers.OAuthInboundAuthType,
				OAuthConfig: oauthProfileToComplete(clientID, oauth),
			},
		}
	}

	if clientID != "" {
		oauthCert, oauthCertOpErr := s.inboundClientService.GetCertificate(
			ctx, cert.CertificateReferenceTypeOAuthApp, clientID)
		if oauthCertOpErr != nil {
			return nil, s.translateCertOperationError(ctx, oauthCertOpErr)
		}
		if len(resp.InboundAuthConfig) > 0 && resp.InboundAuthConfig[0].OAuthConfig != nil {
			resp.InboundAuthConfig[0].OAuthConfig.Certificate = oauthCert
		}
	}

	return resp, nil
}

// buildListResponse builds the paged agent list response from a slice of entities and pagination metadata.
func (s *agentService) buildListResponse(ctx context.Context, entities []providers.Entity,
	totalCount, limit, offset int, includeDisplay bool) *model.AgentListResponse {
	agents := make([]model.BasicAgentResponse, 0, len(entities))
	for i := range entities {
		e := &entities[i]
		name, description, owner, clientID := readSystemAttributes(e.SystemAttributes)
		agents = append(agents, model.BasicAgentResponse{
			ID:          e.ID,
			OUID:        e.OUID,
			OUHandle:    e.OUHandle,
			Type:        e.Type,
			Name:        name,
			Description: description,
			ClientID:    clientID,
			Owner:       owner,
			Attributes:  e.Attributes,
			IsReadOnly:  e.IsReadOnly,
		})
	}

	if includeDisplay {
		s.populateOUHandlesForList(ctx, agents)
	}

	displayQuery := sysutils.DisplayQueryParam(includeDisplay)
	return &model.AgentListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(agents),
		Agents:       agents,
		Links:        sysutils.BuildPaginationLinks(agentBasePath, limit, offset, totalCount, displayQuery),
	}
}

// lookupOUHandle resolves the OU handle for the given OU ID; returns empty string on failure.
func (s *agentService) lookupOUHandle(ctx context.Context, ouID string) string {
	handles, err := s.ouService.GetOrganizationUnitHandlesByIDs(ctx, []string{ouID})
	if err != nil {
		s.logger.Debug(ctx, "Failed to resolve OU handle for agent",
			log.String("ouID", ouID), log.Any("error", err))
		return ""
	}
	return handles[ouID]
}

// populateOUHandleForGet resolves and sets OUHandle on a single-agent GET response; silently skips on lookup failure.
func (s *agentService) populateOUHandleForGet(ctx context.Context, resp *model.AgentGetResponse) {
	if resp.OUID == "" || resp.OUHandle != "" {
		return
	}
	resp.OUHandle = s.lookupOUHandle(ctx, resp.OUID)
}

// populateOUHandleForComplete sets OUHandle on a complete-agent response; silently skips on lookup failure.
func (s *agentService) populateOUHandleForComplete(ctx context.Context, resp *model.AgentCompleteResponse) {
	if resp.OUID == "" {
		return
	}
	resp.OUHandle = s.lookupOUHandle(ctx, resp.OUID)
}

// populateOUHandlesForList batch-resolves OU handles for a list of agents, filling in OUHandle where available.
func (s *agentService) populateOUHandlesForList(ctx context.Context, agents []model.BasicAgentResponse) {
	if len(agents) == 0 {
		return
	}
	idSet := make(map[string]struct{}, len(agents))
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.OUID == "" {
			continue
		}
		if _, ok := idSet[a.OUID]; ok {
			continue
		}
		idSet[a.OUID] = struct{}{}
		ids = append(ids, a.OUID)
	}
	if len(ids) == 0 {
		return
	}
	handles, err := s.ouService.GetOrganizationUnitHandlesByIDs(ctx, ids)
	if err != nil {
		s.logger.Debug(ctx, "Failed to resolve OU handles for agent list", log.Any("error", err))
		return
	}
	for i := range agents {
		if h, ok := handles[agents[i].OUID]; ok {
			agents[i].OUHandle = h
		}
	}
}

// needsInboundClient reports whether any inbound auth field in the create request requires an inbound client row.
func needsInboundClient(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	return agent.AuthFlowID != "" ||
		agent.RegistrationFlowID != "" ||
		agent.IsRegistrationFlowEnabled ||
		agent.ThemeID != "" ||
		agent.LayoutID != "" ||
		agent.Assertion != nil ||
		agent.LoginConsent != nil ||
		len(agent.AllowedUserTypes) > 0 ||
		len(agent.InboundAuthConfig) > 0
}

// updateNeedsInboundClient reports whether an update request contains any inbound auth field requiring a client row.
func updateNeedsInboundClient(req *model.UpdateAgentRequest) bool {
	if req == nil {
		return false
	}
	return req.AuthFlowID != "" ||
		req.RegistrationFlowID != "" ||
		req.IsRegistrationFlowEnabled ||
		req.ThemeID != "" ||
		req.LayoutID != "" ||
		req.Assertion != nil ||
		req.LoginConsent != nil ||
		len(req.AllowedUserTypes) > 0 ||
		len(req.InboundAuthConfig) > 0
}

// validateBaseFields validates the mandatory top-level fields required for both create and update.
func validateBaseFields(name, agentType string) *tidcommon.ServiceError {
	if name == "" {
		return &ErrorInvalidAgentName
	}
	if agentType == "" {
		return &ErrorInvalidAgentType
	}
	return nil
}

// validatePaginationParams validates that limit and offset are within acceptable bounds.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 0 || limit > 100 {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// normalizeLoginConsent clamps a negative ValidityPeriod to zero; leaves a nil config untouched.
func normalizeLoginConsent(lc *inboundmodel.LoginConsentConfig) {
	if lc == nil {
		return
	}
	if lc.ValidityPeriod < 0 {
		lc.ValidityPeriod = 0
	}
}

// pickOAuthConfig returns the single OAuth-typed entry from a request input, or nil if absent.
// Returns ErrorMultipleOAuthConfigs if more than one OAuth entry is present.
func pickOAuthConfig(
	configs []providers.InboundAuthConfigWithSecret,
) (*providers.OAuthConfigWithSecret, *tidcommon.ServiceError) {
	var found *providers.OAuthConfigWithSecret
	isOAuthConfig := false
	for i := range configs {
		if configs[i].Type != providers.OAuthInboundAuthType {
			continue
		}
		if isOAuthConfig {
			return nil, &ErrorMultipleOAuthConfigs
		}
		isOAuthConfig = true
		if configs[i].OAuthConfig != nil {
			found = configs[i].OAuthConfig
		}
	}
	return found, nil
}

// requiresClientSecret reports whether the OAuth config implies a confidential client requiring a secret.
func requiresClientSecret(cfg *providers.OAuthConfigWithSecret) bool {
	if cfg == nil {
		return false
	}
	if cfg.PublicClient {
		return false
	}
	switch cfg.TokenEndpointAuthMethod {
	case providers.TokenEndpointAuthMethodClientSecretBasic,
		providers.TokenEndpointAuthMethodClientSecretPost:
		return true
	case providers.TokenEndpointAuthMethodNone,
		providers.TokenEndpointAuthMethodPrivateKeyJWT:
		return false
	}
	// Default to client_secret_basic when unspecified.
	return true
}

// buildAgentEntity constructs the entity row and system credentials JSON for a new or updated agent.
func buildAgentEntity(agentID, agentType, ouID string, attributes json.RawMessage,
	name, description, owner, clientID, clientSecret string) (*providers.Entity, json.RawMessage, error) {
	sysAttrsJSON, err := buildSystemAttributesJSON(name, description, owner, clientID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build agent system attributes: %w", err)
	}

	sysCredsJSON, err := buildSystemCredentialsJSON(clientSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build agent system credentials: %w", err)
	}

	e := &providers.Entity{
		ID:               agentID,
		Category:         providers.EntityCategoryAgent,
		Type:             agentType,
		State:            providers.EntityStateActive,
		OUID:             ouID,
		Attributes:       attributes,
		SystemAttributes: sysAttrsJSON,
	}
	return e, sysCredsJSON, nil
}

// buildSystemAttributesJSON serializes agent name, description, owner, and clientID into the systemAttributes blob.
func buildSystemAttributesJSON(name, description, owner, clientID string) (json.RawMessage, error) {
	attrs := map[string]interface{}{}
	if name != "" {
		attrs[fieldName] = name
	}
	if description != "" {
		attrs[fieldDescription] = description
	}
	if owner != "" {
		attrs[fieldOwner] = owner
	}
	if clientID != "" {
		attrs[fieldClientID] = clientID
	}
	if len(attrs) == 0 {
		return nil, nil
	}
	return json.Marshal(attrs)
}

// buildSystemCredentialsJSON serializes the client secret into the systemCredentials JSON blob; returns nil when empty.
func buildSystemCredentialsJSON(clientSecret string) (json.RawMessage, error) {
	if clientSecret == "" {
		return nil, nil
	}
	return json.Marshal(map[string]interface{}{
		fieldClientSecret: clientSecret,
	})
}

// readSystemAttributes deserializes the systemAttributes JSON blob back into individual string fields.
func readSystemAttributes(raw json.RawMessage) (name, description, owner, clientID string) {
	if len(raw) == 0 {
		return "", "", "", ""
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return "", "", "", ""
	}
	if v, ok := attrs[fieldName].(string); ok {
		name = v
	}
	if v, ok := attrs[fieldDescription].(string); ok {
		description = v
	}
	if v, ok := attrs[fieldOwner].(string); ok {
		owner = v
	}
	if v, ok := attrs[fieldClientID].(string); ok {
		clientID = v
	}
	return name, description, owner, clientID
}

// buildInboundClientRecord constructs an InboundClient record from the agent's identity and inbound auth fields.
func buildInboundClientRecord(agentID, authFlowID, regFlowID string, isRegEnabled bool,
	themeID, layoutID string, assertion *inboundmodel.AssertionConfig,
	loginConsent *inboundmodel.LoginConsentConfig, allowedUserTypes []string) inboundmodel.InboundClient {
	return inboundmodel.InboundClient{
		ID:                        agentID,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        regFlowID,
		IsRegistrationFlowEnabled: isRegEnabled,
		ThemeID:                   themeID,
		LayoutID:                  layoutID,
		Assertion:                 assertion,
		LoginConsent:              loginConsent,
		AllowedUserTypes:          allowedUserTypes,
	}
}

// buildOAuthProfile maps the agent OAuth config to the inbound client profile shape.
func buildOAuthProfile(configs []providers.InboundAuthConfigWithSecret) *providers.OAuthProfile {
	cfg, _ := pickOAuthConfig(configs)
	if cfg == nil {
		return nil
	}
	authMethod := cfg.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = providers.TokenEndpointAuthMethodClientSecretBasic
	}
	grantTypes := sysutils.ConvertToStringSlice(cfg.GrantTypes)
	if len(grantTypes) == 0 {
		// Default to client_credentials for agents.
		grantTypes = []string{string(providers.GrantTypeClientCredentials)}
	}
	return &providers.OAuthProfile{
		RedirectURIs:                       cfg.RedirectURIs,
		PostLogoutRedirectURIs:             cfg.PostLogoutRedirectURIs,
		GrantTypes:                         grantTypes,
		ResponseTypes:                      sysutils.ConvertToStringSlice(cfg.ResponseTypes),
		TokenEndpointAuthMethod:            string(authMethod),
		PKCERequired:                       cfg.PKCERequired,
		PublicClient:                       cfg.PublicClient,
		RequirePushedAuthorizationRequests: cfg.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              cfg.DPoPBoundAccessTokens,
		IncludeActClaim:                    cfg.IncludeActClaim,
		Certificate:                        cfg.Certificate,
		Token:                              cfg.Token,
		Scopes:                             cfg.Scopes,
		UserInfo:                           cfg.UserInfo,
		ScopeClaims:                        cfg.ScopeClaims,
	}
}

// oauthProfileToComplete converts a stored OAuth profile into the create/update shape.
func oauthProfileToComplete(clientID string, p *providers.OAuthProfile) *providers.OAuthConfigWithSecret {
	if p == nil {
		return nil
	}
	grants, respTypes := convertGrantAndResponseTypes(p)
	return &providers.OAuthConfigWithSecret{
		ClientID:                           clientID,
		RedirectURIs:                       p.RedirectURIs,
		PostLogoutRedirectURIs:             p.PostLogoutRedirectURIs,
		GrantTypes:                         grants,
		ResponseTypes:                      respTypes,
		TokenEndpointAuthMethod:            providers.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod),
		PKCERequired:                       p.PKCERequired,
		PublicClient:                       p.PublicClient,
		RequirePushedAuthorizationRequests: p.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              p.DPoPBoundAccessTokens,
		IncludeActClaim:                    p.IncludeActClaim,
		Certificate:                        p.Certificate,
		Token:                              p.Token,
		Scopes:                             p.Scopes,
		UserInfo:                           p.UserInfo,
		ScopeClaims:                        p.ScopeClaims,
	}
}

// convertGrantAndResponseTypes adapts the stored string slices to the typed enums shared by
// both response shapes.
func convertGrantAndResponseTypes(
	p *providers.OAuthProfile,
) ([]providers.GrantType, []providers.ResponseType) {
	grants := make([]providers.GrantType, 0, len(p.GrantTypes))
	for _, g := range p.GrantTypes {
		grants = append(grants, providers.GrantType(g))
	}
	respTypes := make([]providers.ResponseType, 0, len(p.ResponseTypes))
	for _, r := range p.ResponseTypes {
		respTypes = append(respTypes, providers.ResponseType(r))
	}
	return grants, respTypes
}

// buildCompleteResponse constructs the full create/update response including credentials and all inbound auth fields.
func buildCompleteResponse(agentID, owner, clientID, clientSecret, agentType, name, description string,
	attributes json.RawMessage, authFlowID, regFlowID string, isRegEnabled bool,
	themeID, layoutID string, assertion *inboundmodel.AssertionConfig,
	loginConsent *inboundmodel.LoginConsentConfig, allowedUserTypes []string,
	inboundAuthConfig []providers.InboundAuthConfigWithSecret,
) *model.AgentCompleteResponse {
	resp := &model.AgentCompleteResponse{
		ID:          agentID,
		Type:        agentType,
		Name:        name,
		Description: description,
		Owner:       owner,
		Attributes:  attributes,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                authFlowID,
			RegistrationFlowID:        regFlowID,
			IsRegistrationFlowEnabled: isRegEnabled,
			ThemeID:                   themeID,
			LayoutID:                  layoutID,
			Assertion:                 assertion,
			LoginConsent:              loginConsent,
			AllowedUserTypes:          allowedUserTypes,
		},
	}
	if len(inboundAuthConfig) > 0 {
		resp.InboundAuthConfig = annotateOAuthConfig(inboundAuthConfig, clientID, clientSecret)
	}
	return resp
}

// annotateOAuthConfig stamps clientID and clientSecret onto the OAuth entry.
func annotateOAuthConfig(
	in []providers.InboundAuthConfigWithSecret, clientID, clientSecret string,
) []providers.InboundAuthConfigWithSecret {
	out := make([]providers.InboundAuthConfigWithSecret, len(in))
	for i, cfg := range in {
		copyCfg := cfg
		if copyCfg.Type == providers.OAuthInboundAuthType && copyCfg.OAuthConfig != nil {
			c := *copyCfg.OAuthConfig
			if clientID != "" {
				c.ClientID = clientID
			}
			if clientSecret != "" {
				c.ClientSecret = clientSecret
			}
			copyCfg.OAuthConfig = &c
		}
		out[i] = copyCfg
	}
	return out
}

// mapEntityError maps entity-layer errors to agent-service errors.
func mapEntityError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, entity.ErrEntityNotFound):
		return &ErrorAgentNotFound
	case errors.Is(err, entity.ErrSchemaValidationFailed):
		return &ErrorSchemaValidationFailed
	case errors.Is(err, entity.ErrAttributeConflict):
		return &ErrorAttributeConflict
	case errors.Is(err, entity.ErrInvalidCredential):
		return &ErrorInvalidCredential
	}
	return nil
}

// translateInboundClientError maps inbound-client-layer errors to agent-service errors.
func (s *agentService) translateInboundClientError(ctx context.Context, err error) *tidcommon.ServiceError {
	if err == nil {
		return nil
	}
	if errors.Is(err, inboundclient.ErrCannotModifyDeclarative) {
		return &ErrorCannotModifyDeclarativeResource
	}
	if svcErr := translateInboundClientFKError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateOAuthValidationError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateUserInfoValidationError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateIDTokenValidationError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateCertValidationError(err); svcErr != nil {
		return svcErr
	}
	var opErr *inboundclient.CertOperationError
	if errors.As(err, &opErr) {
		return s.translateCertOperationError(ctx, opErr)
	}
	return nil
}

// translateOAuthValidationError maps OAuth redirect URI, grant type, response type,
// token endpoint auth method, and public client validation errors to agent-service errors.
func translateOAuthValidationError(err error) *tidcommon.ServiceError {
	switch {
	// OAuth: redirect URI
	case errors.Is(err, inboundclient.ErrOAuthInvalidRedirectURI):
		return &ErrorInvalidRedirectURI
	case errors.Is(err, inboundclient.ErrOAuthRedirectURIFragmentNotAllowed):
		return tidcommon.CustomServiceError(ErrorInvalidRedirectURI, tidcommon.I18nMessage{
			Key:          "error.agentservice.redirect_uri_fragment_not_allowed_description",
			DefaultValue: "Redirect URIs must not contain a fragment component",
		})
	case errors.Is(err, inboundclient.ErrOAuthAuthCodeRequiresRedirectURIs):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.auth_code_requires_redirect_uris_description",
			DefaultValue: "authorization_code grant type requires redirect URIs",
		})

	// OAuth: grant + response type
	case errors.Is(err, inboundclient.ErrOAuthInvalidGrantType):
		return &ErrorInvalidGrantType
	case errors.Is(err, inboundclient.ErrOAuthInvalidResponseType):
		return &ErrorInvalidResponseType
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseResponseTypes):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.client_credentials_cannot_use_response_types_description",
			DefaultValue: "client_credentials grant type cannot be used with response types",
		})
	case errors.Is(err, inboundclient.ErrOAuthAuthCodeRequiresCodeResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.auth_code_requires_code_response_type_description",
			DefaultValue: "authorization_code grant type requires 'code' response type",
		})
	case errors.Is(err, inboundclient.ErrOAuthRefreshTokenCannotBeSoleGrant):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.refresh_token_cannot_be_sole_grant_description",
			DefaultValue: "refresh_token grant type cannot be used without another grant type",
		})
	case errors.Is(err, inboundclient.ErrOAuthPKCERequiresAuthCode):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.pkce_requires_authorization_code_description",
			DefaultValue: "PKCE can only be enabled when the authorization_code grant type is selected",
		})
	case errors.Is(err, inboundclient.ErrOAuthResponseTypesRequireAuthCode):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.response_types_require_authorization_code_description",
			DefaultValue: "Response types can only be configured with the authorization_code grant type",
		})

	// OAuth: token endpoint auth method
	case errors.Is(err, inboundclient.ErrOAuthInvalidTokenEndpointAuthMethod):
		return &ErrorInvalidTokenEndpointAuthMethod
	case errors.Is(err, inboundclient.ErrOAuthPrivateKeyJWTRequiresCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.private_key_jwt_requires_certificate_description",
			DefaultValue: "private_key_jwt authentication method requires a certificate",
		})
	case errors.Is(err, inboundclient.ErrOAuthCertificateRequiresClientID):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.certificate_requires_client_id_description",
			DefaultValue: "certificate configuration requires an OAuth client ID",
		})
	case errors.Is(err, inboundclient.ErrOAuthPrivateKeyJWTCannotHaveClientSecret):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.private_key_jwt_cannot_have_client_secret_description",
			DefaultValue: "private_key_jwt authentication method cannot have a client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientSecretCannotHaveCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.client_secret_cannot_have_certificate_description",
			DefaultValue: "client_secret authentication methods cannot have a certificate",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthRequiresPublicClient):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.none_auth_method_requires_public_client_description",
			DefaultValue: "'none' authentication method requires the client to be a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthCannotHaveCertOrSecret):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.none_auth_method_cannot_have_cert_or_secret_description",
			DefaultValue: "'none' authentication method cannot have a certificate or client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.client_credentials_cannot_use_none_auth_description",
			DefaultValue: "client_credentials grant type cannot use 'none' authentication method",
		})

	// OAuth: public client
	case errors.Is(err, inboundclient.ErrOAuthPublicClientMustUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidPublicClientConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.public_client_must_use_none_auth_description",
			DefaultValue: "Public clients must use 'none' as token endpoint authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthPublicClientMustHavePKCE):
		return tidcommon.CustomServiceError(ErrorInvalidPublicClientConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.public_client_must_have_pkce_description",
			DefaultValue: "Public clients must have PKCE required set to true",
		})
	}
	return nil
}

// translateUserInfoValidationError maps OAuth userinfo validation errors to agent-service errors.
func translateUserInfoValidationError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedSigningAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_signing_alg_description",
			DefaultValue: "userinfo signing algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedEncryptionAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_encryption_alg_description",
			DefaultValue: "userinfo encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedEncryptionEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_encryption_enc_description",
			DefaultValue: "userinfo content-encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionAlgRequiresEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_encryption_alg_requires_enc_description",
			DefaultValue: "userinfo encryptionEnc is required when encryptionAlg is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionEncRequiresAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_encryption_enc_requires_alg_description",
			DefaultValue: "userinfo encryptionAlg is required when encryptionEnc is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionRequiresCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_encryption_requires_certificate_description",
			DefaultValue: "a certificate (JWKS or JWKS_URI) is required when userinfo encryption is configured",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWKSURINotSSRFSafe):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_jwks_uri_not_ssrf_safe_description",
			DefaultValue: "userinfo JWKS URI must be a publicly reachable HTTPS URL",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_response_type_description",
			DefaultValue: "userinfo responseType is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWSRequiresSigningAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_jws_requires_signing_alg_description",
			DefaultValue: "signingAlg is required when userinfo responseType is JWS",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWERequiresEncryption):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_jwe_requires_encryption_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required when userinfo responseType is JWE",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.userinfo_nested_jwt_requires_all_description",
			DefaultValue: "signingAlg, encryptionAlg, and encryptionEnc are required " +
				"when userinfo responseType is NESTED_JWT",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoAlgRequiresResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_alg_requires_response_type_description",
			DefaultValue: "userinfo responseType is required when signingAlg or encryptionAlg is set",
		})
	}
	return nil
}

// translateIDTokenValidationError maps OAuth ID token validation errors to agent-service errors.
func translateIDTokenValidationError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionFieldsNotAllowed):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_fields_not_allowed_description",
			DefaultValue: "idToken encryptionAlg and encryptionEnc must not be set when responseType is JWT",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_response_type_description",
			DefaultValue: "ID token responseType is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedEncryptionAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_encryption_alg_description",
			DefaultValue: "ID token encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedEncryptionEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_encryption_enc_description",
			DefaultValue: "ID token content-encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionAlgRequiresEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_alg_requires_enc_description",
			DefaultValue: "idToken encryptionEnc is required when encryptionAlg is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionEncRequiresAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_enc_requires_alg_description",
			DefaultValue: "idToken encryptionAlg is required when encryptionEnc is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionRequiresCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_requires_certificate_description",
			DefaultValue: "a certificate (JWKS or JWKS_URI) is required when ID token encryption is configured",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenJWKSURINotSSRFSafe):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_jwks_uri_not_ssrf_safe_description",
			DefaultValue: "idToken JWKS URI must be a publicly reachable HTTPS URL",
		})
	}
	return nil
}

// translateInboundClientFKError maps foreign-key reference errors to agent-service errors.
func translateInboundClientFKError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrFKInvalidAuthFlow):
		return &ErrorInvalidAuthFlowID
	case errors.Is(err, inboundclient.ErrFKInvalidRegistrationFlow):
		return &ErrorInvalidRegistrationFlowID
	case errors.Is(err, inboundclient.ErrFKFlowDefinitionRetrievalFailed):
		return &ErrorWhileRetrievingFlowDefinition
	case errors.Is(err, inboundclient.ErrFKFlowServerError):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrFKThemeNotFound):
		return &ErrorThemeNotFound
	case errors.Is(err, inboundclient.ErrFKLayoutNotFound):
		return &ErrorLayoutNotFound
	case errors.Is(err, inboundclient.ErrFKInvalidUserType):
		return &ErrorInvalidUserType
	case errors.Is(err, inboundclient.ErrUserSchemaLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrInvalidUserAttribute):
		return &ErrorInvalidUserAttribute
	}
	return nil
}

// translateCertValidationError maps certificate validation errors to agent-service errors.
func translateCertValidationError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrCertValueRequired):
		return &ErrorInvalidCertificateValue
	case errors.Is(err, inboundclient.ErrCertInvalidJWKSURI):
		return &ErrorInvalidJWKSURI
	case errors.Is(err, inboundclient.ErrCertInvalidType):
		return &ErrorInvalidCertificateType
	}
	return nil
}

// translateCertOperationError maps a typed CertOperationError to an agent-service error.
func (s *agentService) translateCertOperationError(
	ctx context.Context, err *inboundclient.CertOperationError) *tidcommon.ServiceError {
	if !err.IsClientError() {
		s.logger.Error(ctx, "Certificate operation failed",
			log.Any("operation", err.Operation),
			log.Any("refType", err.RefType),
			log.Any("serviceError", err.Underlying))
		return &tidcommon.InternalServerError
	}
	var key, prefix string
	switch err.Operation {
	case inboundclient.CertOpCreate:
		key, prefix = "error.agentservice.create_certificate_failed_description",
			"Failed to create agent certificate: "
	case inboundclient.CertOpUpdate:
		key, prefix = "error.agentservice.update_certificate_failed_description",
			"Failed to update agent certificate: "
	case inboundclient.CertOpRetrieve:
		key, prefix = "error.agentservice.retrieve_certificate_failed_description",
			"Failed to retrieve agent certificate: "
	case inboundclient.CertOpDelete:
		if err.RefType == cert.CertificateReferenceTypeOAuthApp {
			key, prefix = "error.agentservice.delete_oauth_certificate_failed_description",
				"Failed to delete OAuth app certificate: "
		} else {
			key, prefix = "error.agentservice.delete_certificate_failed_description",
				"Failed to delete agent certificate: "
		}
	default:
		return &tidcommon.InternalServerError
	}
	return tidcommon.CustomServiceError(ErrorCertificateClientError, tidcommon.I18nMessage{
		Key:          key,
		DefaultValue: prefix + err.Underlying.ErrorDescription.DefaultValue,
	})
}
