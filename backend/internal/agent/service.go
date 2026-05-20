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

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// AgentServiceInterface defines the operations exposed by the agent service.
type AgentServiceInterface interface {
	CreateAgent(ctx context.Context, req *model.CreateAgentRequest) (*model.AgentCompleteResponse,
		*serviceerror.ServiceError)
	GetAgent(ctx context.Context, agentID string, includeDisplay bool) (*model.AgentGetResponse,
		*serviceerror.ServiceError)
	GetAgentList(ctx context.Context, limit, offset int, filters map[string]interface{},
		includeDisplay bool) (*model.AgentListResponse, *serviceerror.ServiceError)
	UpdateAgent(ctx context.Context, agentID string, req *model.UpdateAgentRequest) (
		*model.AgentCompleteResponse, *serviceerror.ServiceError)
	DeleteAgent(ctx context.Context, agentID string) *serviceerror.ServiceError
	GetAgentGroups(ctx context.Context, agentID string, limit, offset int) (
		*model.AgentGroupListResponse, *serviceerror.ServiceError)
}

type agentService struct {
	logger               *log.Logger
	entityService        entity.EntityServiceInterface
	inboundClientService inboundclient.InboundClientServiceInterface
	ouService            oupkg.OrganizationUnitServiceInterface
}

func newAgentService(
	entityService entity.EntityServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
) AgentServiceInterface {
	return &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		entityService:        entityService,
		inboundClientService: inboundClientService,
		ouService:            ouService,
	}
}

// CreateAgent creates an agent entity with optional inbound auth profile.
func (s *agentService) CreateAgent(ctx context.Context, req *model.CreateAgentRequest) (
	*model.AgentCompleteResponse, *serviceerror.ServiceError) {
	if req == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if svcErr := validateBaseFields(req.Name, req.Type); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.validateOUExists(ctx, req.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.validateNameUnique(ctx, req.Name, ""); svcErr != nil {
		return nil, svcErr
	}

	agentID, err := sysutils.GenerateUUIDv7()
	if err != nil {
		s.logger.Error("Failed to generate agent ID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	normalizeLoginConsent(req.LoginConsent)

	clientID, clientSecret, svcErr := s.resolveOAuthCredentials(ctx, req.InboundAuthConfig, "", "")
	if svcErr != nil {
		return nil, svcErr
	}

	owner := req.Owner
	if owner == "" {
		// Default to the authenticated subject.
		owner = security.GetSubject(ctx)
	} else if svcErr := s.validateOwnerExists(ctx, owner); svcErr != nil {
		return nil, svcErr
	}

	e, sysCredsJSON, buildErr := buildAgentEntity(agentID, req.Type, req.OUID, req.Attributes,
		req.Name, req.Description, owner, clientID, clientSecret)
	if buildErr != nil {
		s.logger.Error("Failed to build agent entity", log.Error(buildErr))
		return nil, &serviceerror.InternalServerError
	}

	createdEntity, entErr := s.entityService.CreateEntity(ctx, e, sysCredsJSON)
	if entErr != nil {
		if mapped := mapEntityError(entErr); mapped != nil {
			return nil, mapped
		}
		s.logger.Error("Failed to create agent entity", log.String("agentID", agentID), log.Error(entErr))
		return nil, &serviceerror.InternalServerError
	}

	authFlowID, regFlowID := req.AuthFlowID, req.RegistrationFlowID
	assertion, loginConsent := req.Assertion, req.LoginConsent
	var inboundConfigs []inboundmodel.InboundAuthConfigWithSecret

	if needsInboundClient(req) {
		resolvedClient, resolvedOAuth, svcErr := s.createInboundForAgent(ctx, agentID, req, clientSecret)
		if svcErr != nil {
			s.deleteEntityCompensation(ctx, agentID)
			return nil, svcErr
		}
		authFlowID = resolvedClient.AuthFlowID
		regFlowID = resolvedClient.RegistrationFlowID
		assertion = resolvedClient.Assertion
		loginConsent = resolvedClient.LoginConsent
		if resolvedOAuth != nil {
			inboundConfigs = []inboundmodel.InboundAuthConfigWithSecret{{
				Type:        inboundmodel.OAuthInboundAuthType,
				OAuthConfig: oauthProfileToComplete(clientID, resolvedOAuth),
			}}
		}
	}

	resp := buildCompleteResponse(agentID, owner, clientID, clientSecret,
		req.Type, req.Name, req.Description, createdEntity.Attributes,
		authFlowID, regFlowID, req.IsRegistrationFlowEnabled,
		req.ThemeID, req.LayoutID, assertion, loginConsent,
		req.AllowedUserTypes, req.Certificate, inboundConfigs)
	resp.OUID = req.OUID
	s.populateOUHandleForComplete(ctx, resp)
	return resp, nil
}

// GetAgent returns a single agent by ID.
func (s *agentService) GetAgent(ctx context.Context, agentID string, includeDisplay bool) (
	*model.AgentGetResponse, *serviceerror.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}

	e, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error("Failed to retrieve agent entity", log.String("agentID", agentID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if e.Category != entity.EntityCategoryAgent {
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
	*model.AgentListResponse, *serviceerror.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = 30
	}

	totalCount, err := s.entityService.GetEntityListCount(ctx, entity.EntityCategoryAgent, filters)
	if err != nil {
		s.logger.Error("Failed to get agent list count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	entities, err := s.entityService.GetEntityList(ctx, entity.EntityCategoryAgent, limit, offset, filters)
	if err != nil {
		s.logger.Error("Failed to get agent list", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return s.buildListResponse(ctx, entities, totalCount, limit, offset, includeDisplay), nil
}

// UpdateAgent applies a full-replacement update to the agent.
func (s *agentService) UpdateAgent(ctx context.Context, agentID string,
	req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *serviceerror.ServiceError) {
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
		s.logger.Error("Failed to retrieve agent entity for update",
			log.String("agentID", agentID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if existing.Category != entity.EntityCategoryAgent {
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
		s.logger.Error("Failed to load existing OAuth profile",
			log.String("agentID", agentID), log.Error(oauthErr))
		return nil, &serviceerror.InternalServerError
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

	ouID := req.OUID
	if ouID == "" {
		ouID = existing.OUID
	} else if ouID != existing.OUID {
		if svcErr := s.validateOUExists(ctx, ouID); svcErr != nil {
			return nil, svcErr
		}
	}

	resolvedClient, resolvedOAuth, svcErr := s.reconcileInboundForUpdate(
		ctx, agentID, req, clientID, clientSecret, currentName, req.Name)
	if svcErr != nil {
		return nil, svcErr
	}

	if resolvedOAuth == nil {
		clientID = ""
		clientSecret = ""
	}

	updatedEntity := &entity.Entity{
		ID:         agentID,
		Category:   entity.EntityCategoryAgent,
		Type:       req.Type,
		State:      entity.EntityStateActive,
		OUID:       ouID,
		Attributes: req.Attributes,
	}
	sysAttrsJSON, marshalErr := buildSystemAttributesJSON(req.Name, req.Description, owner, clientID)
	if marshalErr != nil {
		s.logger.Error("Failed to build system attributes for update", log.Error(marshalErr))
		return nil, &serviceerror.InternalServerError
	}
	updatedEntity.SystemAttributes = sysAttrsJSON

	if _, err := s.entityService.UpdateEntity(ctx, agentID, updatedEntity); err != nil {
		if mapped := mapEntityError(err); mapped != nil {
			return nil, mapped
		}
		s.logger.Error("Failed to update agent entity", log.String("agentID", agentID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if clientSecret != "" {
		sysCredsJSON, credErr := buildSystemCredentialsJSON(clientSecret)
		if credErr == nil && sysCredsJSON != nil {
			if err := s.entityService.UpdateSystemCredentials(ctx, agentID, sysCredsJSON); err != nil {
				s.logger.Error("Failed to update agent system credentials",
					log.String("agentID", agentID), log.Error(err))
				return nil, &serviceerror.InternalServerError
			}
		}
	}

	authFlowID := resolvedClient.AuthFlowID
	regFlowID := resolvedClient.RegistrationFlowID
	assertion := resolvedClient.Assertion
	loginConsent := resolvedClient.LoginConsent
	var inboundConfigs []inboundmodel.InboundAuthConfigWithSecret
	if resolvedOAuth != nil {
		inboundConfigs = []inboundmodel.InboundAuthConfigWithSecret{{
			Type:        inboundmodel.OAuthInboundAuthType,
			OAuthConfig: oauthProfileToComplete(clientID, resolvedOAuth),
		}}
	}

	resp := buildCompleteResponse(agentID, owner, clientID, clientSecret,
		req.Type, req.Name, req.Description, req.Attributes,
		authFlowID, regFlowID, resolvedClient.IsRegistrationFlowEnabled,
		req.ThemeID, req.LayoutID, assertion, loginConsent,
		req.AllowedUserTypes, req.Certificate, inboundConfigs)
	resp.OUID = ouID
	s.populateOUHandleForComplete(ctx, resp)
	return resp, nil
}

// DeleteAgent removes the agent and its associated inbound client.
func (s *agentService) DeleteAgent(ctx context.Context, agentID string) *serviceerror.ServiceError {
	if agentID == "" {
		return &ErrorMissingAgentID
	}

	existing, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorAgentNotFound
		}
		s.logger.Error("Failed to retrieve agent for delete", log.String("agentID", agentID), log.Error(err))
		return &serviceerror.InternalServerError
	}
	if existing.Category != entity.EntityCategoryAgent {
		return &ErrorAgentNotFound
	}
	if existing.IsReadOnly {
		return &ErrorCannotModifyDeclarativeResource
	}

	if err := s.inboundClientService.DeleteInboundClient(ctx, agentID); err != nil &&
		!errors.Is(err, inboundclient.ErrInboundClientNotFound) {
		if svcErr := s.translateInboundClientError(err); svcErr != nil {
			return svcErr
		}
		s.logger.Error("Failed to delete inbound client for agent",
			log.Error(err), log.String("agentID", agentID))
		return &serviceerror.InternalServerError
	}

	if err := s.entityService.DeleteEntity(ctx, agentID); err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil
		}
		s.logger.Error("Failed to delete agent entity", log.String("agentID", agentID), log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// GetAgentGroups returns the groups the agent belongs to.
func (s *agentService) GetAgentGroups(ctx context.Context, agentID string, limit, offset int) (
	*model.AgentGroupListResponse, *serviceerror.ServiceError) {
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
		s.logger.Error("Failed to retrieve agent for groups", log.String("agentID", agentID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if existing.Category != entity.EntityCategoryAgent {
		return nil, &ErrorAgentNotFound
	}

	totalCount, err := s.entityService.GetGroupCountForEntity(ctx, agentID)
	if err != nil {
		s.logger.Error("Failed to get agent group count", log.String("agentID", agentID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	groups, err := s.entityService.GetEntityGroups(ctx, agentID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get agent groups", log.String("agentID", agentID), log.Error(err))
		return nil, &serviceerror.InternalServerError
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

// deleteEntityCompensation deletes the entity row as a best-effort rollback after a failed downstream operation.
func (s *agentService) deleteEntityCompensation(ctx context.Context, agentID string) {
	if err := s.entityService.DeleteEntity(ctx, agentID); err != nil {
		s.logger.Error("Failed to delete entity during compensation",
			log.String("agentID", agentID), log.Error(err))
	}
}

// validateOUExists returns an error if the given OU is empty or does not exist.
func (s *agentService) validateOUExists(ctx context.Context, ouID string) *serviceerror.ServiceError {
	if ouID == "" {
		return &ErrorOrganizationUnitNotFound
	}
	exists, err := s.ouService.IsOrganizationUnitExists(ctx, ouID)
	if err != nil {
		if err.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return &ErrorOrganizationUnitNotFound
		}
		s.logger.Error("Failed to verify OU existence", log.String("ouID", ouID), log.Any("error", err))
		return &serviceerror.InternalServerError
	}
	if !exists {
		return &ErrorOrganizationUnitNotFound
	}
	return nil
}

// resolveUpdateOwner picks the effective owner for an update — either the requested owner or the
// existing one — and validates it exists when the owner is changing.
func (s *agentService) resolveUpdateOwner(
	ctx context.Context, requestedOwner, currentOwner string,
) (string, *serviceerror.ServiceError) {
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
func (s *agentService) validateOwnerExists(ctx context.Context, ownerID string) *serviceerror.ServiceError {
	if ownerID == "" {
		return nil
	}
	_, err := s.entityService.GetEntity(ctx, ownerID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorOwnerNotFound
		}
		s.logger.Error("Failed to verify owner existence", log.String("ownerID", ownerID), log.Error(err))
		return &serviceerror.InternalServerError
	}
	return nil
}

// validateNameUnique returns an error if another agent already uses the given name (excludeID is exempt on updates).
func (s *agentService) validateNameUnique(ctx context.Context, name, excludeID string) *serviceerror.ServiceError {
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
		s.logger.Error("Failed to verify agent name uniqueness", log.Error(err))
		return &serviceerror.InternalServerError
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
	if getErr != nil || found.Category != entity.EntityCategoryAgent {
		return nil
	}
	return &ErrorAgentAlreadyExistsWithName
}

// resolveOAuthCredentials resolves the clientID and clientSecret for an agent OAuth profile.
func (s *agentService) resolveOAuthCredentials(ctx context.Context,
	configs []inboundmodel.InboundAuthConfigWithSecret, existingClientID, existingOAuthMethod string,
) (string, string, *serviceerror.ServiceError) {
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
			s.logger.Error("Failed to generate client ID", log.Error(err))
			return "", "", &serviceerror.InternalServerError
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
		existingWasSecretBased := existingOAuthMethod == string(oauth2const.TokenEndpointAuthMethodClientSecretBasic) ||
			existingOAuthMethod == string(oauth2const.TokenEndpointAuthMethodClientSecretPost)
		if !existingWasSecretBased {
			generated, err := oauthutils.GenerateOAuth2ClientSecret()
			if err != nil {
				s.logger.Error("Failed to generate client secret", log.Error(err))
				return "", "", &serviceerror.InternalServerError
			}
			clientSecret = generated
		}
	}
	return clientID, clientSecret, nil
}

// isClientIDTaken reports whether the given clientID is already used by a different entity.
func (s *agentService) isClientIDTaken(
	ctx context.Context, clientID, excludeID string) (bool, *serviceerror.ServiceError) {
	id, err := s.entityService.IdentifyEntity(ctx, map[string]interface{}{fieldClientID: clientID})
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return false, nil
		}
		s.logger.Error("Failed to check client ID availability", log.MaskedString("clientID", clientID),
			log.Error(err))
		return false, &serviceerror.InternalServerError
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
	req *model.CreateAgentRequest, clientSecret string) (
	inboundmodel.InboundClient, *inboundmodel.OAuthProfile, *serviceerror.ServiceError) {
	client := buildInboundClientRecord(agentID, req.AuthFlowID, req.RegistrationFlowID,
		req.IsRegistrationFlowEnabled, req.ThemeID, req.LayoutID, req.Assertion,
		req.LoginConsent, req.AllowedUserTypes)

	oauthProfile := buildOAuthProfile(req.InboundAuthConfig)

	hasSecret := clientSecret != ""
	if err := s.inboundClientService.CreateInboundClient(ctx, &client, req.Certificate,
		oauthProfile, hasSecret, req.Name); err != nil {
		if svcErr := s.translateInboundClientError(err); svcErr != nil {
			return inboundmodel.InboundClient{}, nil, svcErr
		}
		s.logger.Error("Failed to create inbound client for agent",
			log.Error(err), log.String("agentID", agentID))
		return inboundmodel.InboundClient{}, nil, &serviceerror.InternalServerError
	}
	return client, oauthProfile, nil
}

// reconcileInboundForUpdate creates, updates, or removes the inbound client row and returns the mutated structs.
func (s *agentService) reconcileInboundForUpdate(ctx context.Context, agentID string,
	req *model.UpdateAgentRequest, clientID, clientSecret, oldName, newName string,
) (inboundmodel.InboundClient, *inboundmodel.OAuthProfile, *serviceerror.ServiceError) {
	wantsInbound := updateNeedsInboundClient(req)

	existingClient, getErr := s.inboundClientService.GetInboundClientByEntityID(ctx, agentID)
	hasExisting := getErr == nil && existingClient != nil

	if !wantsInbound {
		if hasExisting {
			if err := s.inboundClientService.DeleteInboundClient(ctx, agentID); err != nil &&
				!errors.Is(err, inboundclient.ErrInboundClientNotFound) {
				if svcErr := s.translateInboundClientError(err); svcErr != nil {
					return inboundmodel.InboundClient{}, nil, svcErr
				}
				s.logger.Error("Failed to remove inbound client during update",
					log.Error(err), log.String("agentID", agentID))
				return inboundmodel.InboundClient{}, nil, &serviceerror.InternalServerError
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
		entityName := newName
		if entityName == "" {
			entityName = oldName
		}
		if err := s.inboundClientService.UpdateInboundClient(ctx, &client, req.Certificate,
			oauthProfile, hasSecret, clientID, entityName); err != nil {
			if svcErr := s.translateInboundClientError(err); svcErr != nil {
				return inboundmodel.InboundClient{}, nil, svcErr
			}
			s.logger.Error("Failed to update inbound client",
				log.Error(err), log.String("agentID", agentID))
			return inboundmodel.InboundClient{}, nil, &serviceerror.InternalServerError
		}
		return client, oauthProfile, nil
	}

	if err := s.inboundClientService.CreateInboundClient(ctx, &client, req.Certificate,
		oauthProfile, hasSecret, newName); err != nil {
		if svcErr := s.translateInboundClientError(err); svcErr != nil {
			return inboundmodel.InboundClient{}, nil, svcErr
		}
		s.logger.Error("Failed to create inbound client during update",
			log.Error(err), log.String("agentID", agentID))
		return inboundmodel.InboundClient{}, nil, &serviceerror.InternalServerError
	}
	return client, oauthProfile, nil
}

// composeGetResponse builds the GET response by loading inbound client, OAuth profile, and certificates for the entity.
func (s *agentService) composeGetResponse(ctx context.Context, e *entity.Entity) (
	*model.AgentGetResponse, *serviceerror.ServiceError) {
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
			s.logger.Error("Failed to load inbound client for agent",
				log.String("agentID", e.ID), log.Error(err))
			return nil, &serviceerror.InternalServerError
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
		s.logger.Error("Failed to load OAuth profile for agent",
			log.String("agentID", e.ID), log.Error(oauthErr))
		return nil, &serviceerror.InternalServerError
	}
	if oauthErr == nil && oauth != nil {
		resp.InboundAuthConfig = []inboundmodel.InboundAuthConfig{
			{
				Type:        inboundmodel.OAuthInboundAuthType,
				OAuthConfig: oauthProfileToConfig(clientID, oauth),
			},
		}
	}

	entityCert, certOpErr := s.inboundClientService.GetCertificate(ctx, cert.CertificateReferenceTypeApplication, e.ID)
	if certOpErr != nil {
		return nil, s.translateCertOperationError(certOpErr)
	}
	resp.Certificate = entityCert

	if clientID != "" {
		oauthCert, oauthCertOpErr := s.inboundClientService.GetCertificate(
			ctx, cert.CertificateReferenceTypeOAuthApp, clientID)
		if oauthCertOpErr != nil {
			return nil, s.translateCertOperationError(oauthCertOpErr)
		}
		if len(resp.InboundAuthConfig) > 0 && resp.InboundAuthConfig[0].OAuthConfig != nil {
			resp.InboundAuthConfig[0].OAuthConfig.Certificate = oauthCert
		}
	}

	return resp, nil
}

// buildListResponse builds the paged agent list response from a slice of entities and pagination metadata.
func (s *agentService) buildListResponse(ctx context.Context, entities []entity.Entity,
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
		s.logger.Debug("Failed to resolve OU handle for agent", log.String("ouID", ouID), log.Any("error", err))
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
		s.logger.Debug("Failed to resolve OU handles for agent list", log.Any("error", err))
		return
	}
	for i := range agents {
		if h, ok := handles[agents[i].OUID]; ok {
			agents[i].OUHandle = h
		}
	}
}

// needsInboundClient reports whether any inbound auth field in the create request requires an inbound client row.
func needsInboundClient(req *model.CreateAgentRequest) bool {
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
		req.Certificate != nil ||
		len(req.InboundAuthConfig) > 0
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
		req.Certificate != nil ||
		len(req.InboundAuthConfig) > 0
}

// validateBaseFields validates the mandatory top-level fields required for both create and update.
func validateBaseFields(name, agentType string) *serviceerror.ServiceError {
	if name == "" {
		return &ErrorInvalidAgentName
	}
	if agentType == "" {
		return &ErrorInvalidAgentType
	}
	return nil
}

// validatePaginationParams validates that limit and offset are within acceptable bounds.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
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
	configs []inboundmodel.InboundAuthConfigWithSecret,
) (*inboundmodel.OAuthConfigWithSecret, *serviceerror.ServiceError) {
	var found *inboundmodel.OAuthConfigWithSecret
	isOAuthConfig := false
	for i := range configs {
		if configs[i].Type != inboundmodel.OAuthInboundAuthType {
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
func requiresClientSecret(cfg *inboundmodel.OAuthConfigWithSecret) bool {
	if cfg == nil {
		return false
	}
	if cfg.PublicClient {
		return false
	}
	switch cfg.TokenEndpointAuthMethod {
	case oauth2const.TokenEndpointAuthMethodClientSecretBasic,
		oauth2const.TokenEndpointAuthMethodClientSecretPost:
		return true
	case oauth2const.TokenEndpointAuthMethodNone,
		oauth2const.TokenEndpointAuthMethodPrivateKeyJWT:
		return false
	}
	// Default to client_secret_basic when unspecified.
	return true
}

// buildAgentEntity constructs the entity row and system credentials JSON for a new or updated agent.
func buildAgentEntity(agentID, agentType, ouID string, attributes json.RawMessage,
	name, description, owner, clientID, clientSecret string) (*entity.Entity, json.RawMessage, error) {
	sysAttrsJSON, err := buildSystemAttributesJSON(name, description, owner, clientID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build agent system attributes: %w", err)
	}

	sysCredsJSON, err := buildSystemCredentialsJSON(clientSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build agent system credentials: %w", err)
	}

	e := &entity.Entity{
		ID:               agentID,
		Category:         entity.EntityCategoryAgent,
		Type:             agentType,
		State:            entity.EntityStateActive,
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
func buildOAuthProfile(configs []inboundmodel.InboundAuthConfigWithSecret) *inboundmodel.OAuthProfile {
	cfg, _ := pickOAuthConfig(configs)
	if cfg == nil {
		return nil
	}
	authMethod := cfg.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = oauth2const.TokenEndpointAuthMethodClientSecretBasic
	}
	grantTypes := sysutils.ConvertToStringSlice(cfg.GrantTypes)
	if len(grantTypes) == 0 {
		// Default to client_credentials for agents.
		grantTypes = []string{string(oauth2const.GrantTypeClientCredentials)}
	}
	return &inboundmodel.OAuthProfile{
		RedirectURIs:                       cfg.RedirectURIs,
		GrantTypes:                         grantTypes,
		ResponseTypes:                      sysutils.ConvertToStringSlice(cfg.ResponseTypes),
		TokenEndpointAuthMethod:            string(authMethod),
		PKCERequired:                       cfg.PKCERequired,
		PublicClient:                       cfg.PublicClient,
		RequirePushedAuthorizationRequests: cfg.RequirePushedAuthorizationRequests,
		Certificate:                        cfg.Certificate,
		Token:                              cfg.Token,
		Scopes:                             cfg.Scopes,
		UserInfo:                           cfg.UserInfo,
		ScopeClaims:                        cfg.ScopeClaims,
	}
}

// oauthProfileToComplete converts a stored OAuth profile into the create/update shape.
func oauthProfileToComplete(clientID string, p *inboundmodel.OAuthProfile) *inboundmodel.OAuthConfigWithSecret {
	if p == nil {
		return nil
	}
	grants, respTypes := convertGrantAndResponseTypes(p)
	return &inboundmodel.OAuthConfigWithSecret{
		ClientID:                           clientID,
		RedirectURIs:                       p.RedirectURIs,
		GrantTypes:                         grants,
		ResponseTypes:                      respTypes,
		TokenEndpointAuthMethod:            oauth2const.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod),
		PKCERequired:                       p.PKCERequired,
		PublicClient:                       p.PublicClient,
		RequirePushedAuthorizationRequests: p.RequirePushedAuthorizationRequests,
		Certificate:                        p.Certificate,
		Token:                              p.Token,
		Scopes:                             p.Scopes,
		UserInfo:                           p.UserInfo,
		ScopeClaims:                        p.ScopeClaims,
	}
}

// oauthProfileToConfig converts a stored OAuth profile into the read (GET) response shape.
func oauthProfileToConfig(clientID string, p *inboundmodel.OAuthProfile) *inboundmodel.OAuthConfig {
	if p == nil {
		return nil
	}
	grants, respTypes := convertGrantAndResponseTypes(p)
	return &inboundmodel.OAuthConfig{
		ClientID:                           clientID,
		RedirectURIs:                       p.RedirectURIs,
		GrantTypes:                         grants,
		ResponseTypes:                      respTypes,
		TokenEndpointAuthMethod:            oauth2const.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod),
		PKCERequired:                       p.PKCERequired,
		PublicClient:                       p.PublicClient,
		RequirePushedAuthorizationRequests: p.RequirePushedAuthorizationRequests,
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
	p *inboundmodel.OAuthProfile,
) ([]oauth2const.GrantType, []oauth2const.ResponseType) {
	grants := make([]oauth2const.GrantType, 0, len(p.GrantTypes))
	for _, g := range p.GrantTypes {
		grants = append(grants, oauth2const.GrantType(g))
	}
	respTypes := make([]oauth2const.ResponseType, 0, len(p.ResponseTypes))
	for _, r := range p.ResponseTypes {
		respTypes = append(respTypes, oauth2const.ResponseType(r))
	}
	return grants, respTypes
}

// buildCompleteResponse constructs the full create/update response including credentials and all inbound auth fields.
func buildCompleteResponse(agentID, owner, clientID, clientSecret, agentType, name, description string,
	attributes json.RawMessage, authFlowID, regFlowID string, isRegEnabled bool,
	themeID, layoutID string, assertion *inboundmodel.AssertionConfig,
	loginConsent *inboundmodel.LoginConsentConfig, allowedUserTypes []string,
	certificate *inboundmodel.Certificate, inboundAuthConfig []inboundmodel.InboundAuthConfigWithSecret,
) *model.AgentCompleteResponse {
	resp := &model.AgentCompleteResponse{
		ID:          agentID,
		Type:        agentType,
		Name:        name,
		Description: description,
		Owner:       owner,
		Attributes:  attributes,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                authFlowID,
			RegistrationFlowID:        regFlowID,
			IsRegistrationFlowEnabled: isRegEnabled,
			ThemeID:                   themeID,
			LayoutID:                  layoutID,
			Assertion:                 assertion,
			LoginConsent:              loginConsent,
			AllowedUserTypes:          allowedUserTypes,
			Certificate:               certificate,
		},
	}
	if len(inboundAuthConfig) > 0 {
		resp.InboundAuthConfig = annotateOAuthConfig(inboundAuthConfig, clientID, clientSecret)
	}
	return resp
}

// annotateOAuthConfig stamps clientID and clientSecret onto the OAuth entry.
func annotateOAuthConfig(
	in []inboundmodel.InboundAuthConfigWithSecret, clientID, clientSecret string,
) []inboundmodel.InboundAuthConfigWithSecret {
	out := make([]inboundmodel.InboundAuthConfigWithSecret, len(in))
	for i, cfg := range in {
		copyCfg := cfg
		if copyCfg.Type == inboundmodel.OAuthInboundAuthType && copyCfg.OAuthConfig != nil {
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
func mapEntityError(err error) *serviceerror.ServiceError {
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
func (s *agentService) translateInboundClientError(err error) *serviceerror.ServiceError {
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
		return s.translateCertOperationError(opErr)
	}
	var consentErr *inboundclient.ConsentSyncError
	if errors.As(err, &consentErr) {
		return translateConsentSyncError(consentErr)
	}
	return nil
}

// translateOAuthValidationError maps OAuth redirect URI, grant type, response type,
// token endpoint auth method, and public client validation errors to agent-service errors.
func translateOAuthValidationError(err error) *serviceerror.ServiceError {
	switch {
	// OAuth: redirect URI
	case errors.Is(err, inboundclient.ErrOAuthInvalidRedirectURI):
		return &ErrorInvalidRedirectURI
	case errors.Is(err, inboundclient.ErrOAuthRedirectURIFragmentNotAllowed):
		return serviceerror.CustomServiceError(ErrorInvalidRedirectURI, core.I18nMessage{
			Key:          "error.agentservice.redirect_uri_fragment_not_allowed_description",
			DefaultValue: "Redirect URIs must not contain a fragment component",
		})
	case errors.Is(err, inboundclient.ErrOAuthAuthCodeRequiresRedirectURIs):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.auth_code_requires_redirect_uris_description",
			DefaultValue: "authorization_code grant type requires redirect URIs",
		})

	// OAuth: grant + response type
	case errors.Is(err, inboundclient.ErrOAuthInvalidGrantType):
		return &ErrorInvalidGrantType
	case errors.Is(err, inboundclient.ErrOAuthInvalidResponseType):
		return &ErrorInvalidResponseType
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseResponseTypes):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.client_credentials_cannot_use_response_types_description",
			DefaultValue: "client_credentials grant type cannot be used with response types",
		})
	case errors.Is(err, inboundclient.ErrOAuthAuthCodeRequiresCodeResponseType):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.auth_code_requires_code_response_type_description",
			DefaultValue: "authorization_code grant type requires 'code' response type",
		})
	case errors.Is(err, inboundclient.ErrOAuthRefreshTokenCannotBeSoleGrant):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.refresh_token_cannot_be_sole_grant_description",
			DefaultValue: "refresh_token grant type cannot be used without another grant type",
		})
	case errors.Is(err, inboundclient.ErrOAuthPKCERequiresAuthCode):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.pkce_requires_authorization_code_description",
			DefaultValue: "PKCE can only be enabled when the authorization_code grant type is selected",
		})
	case errors.Is(err, inboundclient.ErrOAuthResponseTypesRequireAuthCode):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.response_types_require_authorization_code_description",
			DefaultValue: "Response types can only be configured with the authorization_code grant type",
		})

	// OAuth: token endpoint auth method
	case errors.Is(err, inboundclient.ErrOAuthInvalidTokenEndpointAuthMethod):
		return &ErrorInvalidTokenEndpointAuthMethod
	case errors.Is(err, inboundclient.ErrOAuthPrivateKeyJWTRequiresCertificate):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.private_key_jwt_requires_certificate_description",
			DefaultValue: "private_key_jwt authentication method requires a certificate",
		})
	case errors.Is(err, inboundclient.ErrOAuthPrivateKeyJWTCannotHaveClientSecret):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.private_key_jwt_cannot_have_client_secret_description",
			DefaultValue: "private_key_jwt authentication method cannot have a client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientSecretCannotHaveCertificate):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.client_secret_cannot_have_certificate_description",
			DefaultValue: "client_secret authentication methods cannot have a certificate",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthRequiresPublicClient):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.none_auth_method_requires_public_client_description",
			DefaultValue: "'none' authentication method requires the client to be a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthCannotHaveCertOrSecret):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.none_auth_method_cannot_have_cert_or_secret_description",
			DefaultValue: "'none' authentication method cannot have a certificate or client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.client_credentials_cannot_use_none_auth_description",
			DefaultValue: "client_credentials grant type cannot use 'none' authentication method",
		})

	// OAuth: public client
	case errors.Is(err, inboundclient.ErrOAuthPublicClientMustUseNoneAuth):
		return serviceerror.CustomServiceError(ErrorInvalidPublicClientConfiguration, core.I18nMessage{
			Key:          "error.agentservice.public_client_must_use_none_auth_description",
			DefaultValue: "Public clients must use 'none' as token endpoint authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthPublicClientMustHavePKCE):
		return serviceerror.CustomServiceError(ErrorInvalidPublicClientConfiguration, core.I18nMessage{
			Key:          "error.agentservice.public_client_must_have_pkce_description",
			DefaultValue: "Public clients must have PKCE required set to true",
		})
	}
	return nil
}

// translateUserInfoValidationError maps OAuth userinfo validation errors to agent-service errors.
func translateUserInfoValidationError(err error) *serviceerror.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedSigningAlg):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_signing_alg_description",
			DefaultValue: "userinfo signing algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedEncryptionAlg):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_encryption_alg_description",
			DefaultValue: "userinfo encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedEncryptionEnc):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_encryption_enc_description",
			DefaultValue: "userinfo content-encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionAlgRequiresEnc):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_encryption_alg_requires_enc_description",
			DefaultValue: "userinfo encryptionEnc is required when encryptionAlg is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionEncRequiresAlg):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_encryption_enc_requires_alg_description",
			DefaultValue: "userinfo encryptionAlg is required when encryptionEnc is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionRequiresCertificate):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_encryption_requires_certificate_description",
			DefaultValue: "a certificate (JWKS or JWKS_URI) is required when userinfo encryption is configured",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWKSURINotSSRFSafe):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_jwks_uri_not_ssrf_safe_description",
			DefaultValue: "userinfo JWKS URI must be a publicly reachable HTTPS URL",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedResponseType):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_unsupported_response_type_description",
			DefaultValue: "userinfo responseType is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWSRequiresSigningAlg):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_jws_requires_signing_alg_description",
			DefaultValue: "signingAlg is required when userinfo responseType is JWS",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWERequiresEncryption):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_jwe_requires_encryption_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required when userinfo responseType is JWE",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key: "error.agentservice.userinfo_nested_jwt_requires_all_description",
			DefaultValue: "signingAlg, encryptionAlg, and encryptionEnc are required " +
				"when userinfo responseType is NESTED_JWT",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoAlgRequiresResponseType):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.userinfo_alg_requires_response_type_description",
			DefaultValue: "userinfo responseType is required when signingAlg or encryptionAlg is set",
		})
	}
	return nil
}

// translateIDTokenValidationError maps OAuth ID token validation errors to agent-service errors.
func translateIDTokenValidationError(err error) *serviceerror.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionFieldsNotAllowed):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_fields_not_allowed_description",
			DefaultValue: "idToken encryptionAlg and encryptionEnc must not be set when responseType is JWT",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedResponseType):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_response_type_description",
			DefaultValue: "ID token responseType is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedEncryptionAlg):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_encryption_alg_description",
			DefaultValue: "ID token encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedEncryptionEnc):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_encryption_enc_description",
			DefaultValue: "ID token content-encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionAlgRequiresEnc):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_alg_requires_enc_description",
			DefaultValue: "idToken encryptionEnc is required when encryptionAlg is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionEncRequiresAlg):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_enc_requires_alg_description",
			DefaultValue: "idToken encryptionAlg is required when encryptionEnc is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionRequiresCertificate):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_encryption_requires_certificate_description",
			DefaultValue: "a certificate (JWKS or JWKS_URI) is required when ID token encryption is configured",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenJWKSURINotSSRFSafe):
		return serviceerror.CustomServiceError(ErrorInvalidOAuthConfiguration, core.I18nMessage{
			Key:          "error.agentservice.idtoken_jwks_uri_not_ssrf_safe_description",
			DefaultValue: "idToken JWKS URI must be a publicly reachable HTTPS URL",
		})
	}
	return nil
}

// translateInboundClientFKError maps foreign-key reference errors to agent-service errors.
func translateInboundClientFKError(err error) *serviceerror.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrFKInvalidAuthFlow):
		return &ErrorInvalidAuthFlowID
	case errors.Is(err, inboundclient.ErrFKInvalidRegistrationFlow):
		return &ErrorInvalidRegistrationFlowID
	case errors.Is(err, inboundclient.ErrFKFlowDefinitionRetrievalFailed):
		return &ErrorWhileRetrievingFlowDefinition
	case errors.Is(err, inboundclient.ErrFKFlowServerError):
		return &serviceerror.InternalServerError
	case errors.Is(err, inboundclient.ErrFKThemeNotFound):
		return &ErrorThemeNotFound
	case errors.Is(err, inboundclient.ErrFKLayoutNotFound):
		return &ErrorLayoutNotFound
	case errors.Is(err, inboundclient.ErrFKInvalidUserType):
		return &ErrorInvalidUserType
	case errors.Is(err, inboundclient.ErrUserSchemaLookupFailed):
		return &serviceerror.InternalServerError
	case errors.Is(err, inboundclient.ErrInvalidUserAttribute):
		return &ErrorInvalidUserAttribute
	}
	return nil
}

// translateCertValidationError maps certificate validation errors to agent-service errors.
func translateCertValidationError(err error) *serviceerror.ServiceError {
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
func (s *agentService) translateCertOperationError(err *inboundclient.CertOperationError) *serviceerror.ServiceError {
	if !err.IsClientError() {
		s.logger.Error("Certificate operation failed",
			log.Any("operation", err.Operation),
			log.Any("refType", err.RefType),
			log.Any("serviceError", err.Underlying))
		return &serviceerror.InternalServerError
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
		return &serviceerror.InternalServerError
	}
	return serviceerror.CustomServiceError(ErrorCertificateClientError, core.I18nMessage{
		Key:          key,
		DefaultValue: prefix + err.Underlying.ErrorDescription.DefaultValue,
	})
}

// translateConsentSyncError maps a typed ConsentSyncError to an agent-service error.
func translateConsentSyncError(err *inboundclient.ConsentSyncError) *serviceerror.ServiceError {
	if err.IsClientError() {
		return serviceerror.CustomServiceError(ErrorConsentSyncFailed, core.I18nMessage{
			Key: "error.agentservice.consent_sync_failed_description",
			DefaultValue: fmt.Sprintf(
				ErrorConsentSyncFailed.ErrorDescription.DefaultValue+" : code - %s",
				err.Underlying.Code,
			),
		})
	}
	return &serviceerror.InternalServerError
}
