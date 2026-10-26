// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package agent provides functionality for managing agents
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent/model"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/filter"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// AgentServiceInterface defines the operations exposed by the agent service.
type AgentServiceInterface interface {
	CreateAgent(ctx context.Context, agent *providers.Agent) (*model.AgentCompleteResponse,
		*tidcommon.ServiceError)
	GetAgent(ctx context.Context, agentID string, includeDisplay bool) (*model.AgentGetResponse,
		*tidcommon.ServiceError)
	GetAgentByClientID(ctx context.Context, clientID string, includeDisplay bool) (*model.AgentGetResponse,
		*tidcommon.ServiceError)
	GetAgentList(ctx context.Context, limit, offset int, filters map[string]interface{},
		includeDisplay bool) (*model.AgentListResponse, *tidcommon.ServiceError)
	GetAgentListByProject(ctx context.Context, projectID string, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*model.AgentListResponse, *tidcommon.ServiceError)
	GetAgentListByOwner(ctx context.Context, ownerID string, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*model.AgentListResponse, *tidcommon.ServiceError)
	UpdateAgent(ctx context.Context, agentID string, req *model.UpdateAgentRequest) (
		*model.AgentCompleteResponse, *tidcommon.ServiceError)
	UpdateAgentCredentials(ctx context.Context, agentID string, credentials json.RawMessage) *tidcommon.ServiceError
	RegenerateAgentSecret(ctx context.Context, agentID string) (string, *tidcommon.ServiceError)
	DeleteAgent(ctx context.Context, agentID string) *tidcommon.ServiceError
	GetAgentGroups(ctx context.Context, agentID string, limit, offset int) (
		*model.AgentGroupListResponse, *tidcommon.ServiceError)
	GetAgentRoles(ctx context.Context, agentID string, limit, offset int) (
		*model.AgentRoleListResponse, *tidcommon.ServiceError)
	ValidateAgent(ctx context.Context, agent *providers.Agent, excludeID string) (
		clientID, clientSecret string, client inboundmodel.InboundClient, svcErr *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error)
	SetDependencyRegistry(r resourcedependency.Registry)
	GetClientRules(ctx context.Context, agentID string) (*providers.ClientRules, *tidcommon.ServiceError)
	GetOwnedAgent(ctx context.Context, ownerID, agentID string, includeDisplay bool) (
		*model.AgentGetResponse, *tidcommon.ServiceError)
	GetOwnedAgents(ctx context.Context, ownerID string, kind providers.AgentKind, limit, offset int) (
		*model.AgentListResponse, *tidcommon.ServiceError)
	UpdateOwnedAgent(ctx context.Context, ownerID, agentID string, kind providers.AgentKind,
		req *model.UpdateOwnedAgentRequest) (*model.AgentCompleteResponse, *tidcommon.ServiceError)
	DeleteOwnedAgent(ctx context.Context, ownerID, agentID string, kind providers.AgentKind) *tidcommon.ServiceError
	SuspendOwnedAgent(ctx context.Context, ownerID, agentID string, kind providers.AgentKind) *tidcommon.ServiceError
	UnsuspendOwnedAgent(ctx context.Context, ownerID, agentID string, kind providers.AgentKind) *tidcommon.ServiceError
	SuspendOwnedAgents(ctx context.Context, ownerID string, kind providers.AgentKind) (int, *tidcommon.ServiceError)
}

// projectOUValidator checks that the organization unit of a resource scoped to a project belongs to it.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
}

type agentService struct {
	logger               *log.Logger
	authzService         sysauthz.SystemAuthorizationServiceInterface
	entityService        entity.EntityServiceInterface
	governance           identitygovernance.IdentityGovernanceServiceInterface
	inboundClientService inboundclient.InboundClientServiceInterface
	ouService            oupkg.OrganizationUnitServiceInterface
	dependencyRegistry   resourcedependency.Registry
	roleService          role.RoleServiceInterface
	valueCapturer        declarativeresource.ValueCapturer
	entityTypeService    entitytype.EntityTypeServiceInterface
	// transactioner spans the entity database, so an update's entity write and its lifespan change
	// commit together.
	transactioner    providers.Transactioner
	observabilitySvc observability.ObservabilityServiceInterface
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// projectRequired is set when a deployment that serves projects refuses an agent created without one.
	projectRequired bool
	bus             eventbus.EventBusInterface
	revoker         revocation.CriteriaRevoker
	resolveLifetime artifactLifetimeResolver
}

// artifactLifetimeResolver reports how long an artifact issued to the given OAuth client can remain valid, so a
// revocation of that client's tokens is kept at least that long.
type artifactLifetimeResolver func(client *providers.OAuthClient) time.Duration

func newAgentService(
	authzService sysauthz.SystemAuthorizationServiceInterface,
	entityService entity.EntityServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	roleService role.RoleServiceInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
	entityTypeService entitytype.EntityTypeServiceInterface,
	transactioner providers.Transactioner,
	observabilitySvc observability.ObservabilityServiceInterface,
	projectValidator projectOUValidator,
	bus eventbus.EventBusInterface,
	revoker revocation.CriteriaRevoker,
	artifactLifetime artifactLifetimeResolver,
) AgentServiceInterface {
	return &agentService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentService")),
		authzService:         authzService,
		entityService:        entityService,
		governance:           governance,
		inboundClientService: inboundClientService,
		ouService:            ouService,
		roleService:          roleService,
		valueCapturer:        valueCapturer,
		entityTypeService:    entityTypeService,
		transactioner:        transactioner,
		observabilitySvc:     observabilitySvc,
		projectValidator:     projectValidator,
		projectRequired:      projectValidator != nil && projectscope.ProjectRequired(security.ResourceTypeAgent),
		bus:                  bus,
		revoker:              revoker,
		resolveLifetime:      artifactLifetime,
	}
}

// CreateAgent creates an agent entity with optional inbound auth profile.
func (s *agentService) CreateAgent(ctx context.Context, agent *providers.Agent) (
	*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	created, svcErr := s.createAgent(ctx, agent)
	var agentID string
	if svcErr == nil {
		agentID = created.ID
	}
	s.publishAgentEvent(ctx, audit.EventAgentCreate, audit.ActionCreate, agentID, svcErr)
	return created, svcErr
}

// createAgent creates an agent entity with optional inbound auth profile without publishing its audit
// event.
func (s *agentService) createAgent(ctx context.Context, agent *providers.Agent) (
	*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if agent == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if svcErr := s.checkProjectGiven(ctx, agent.ProjectID); svcErr != nil {
		return nil, svcErr
	}
	normalizeLoginConsent(agent.LoginConsent)

	if svcErr := applyGrantTypes(agent); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := validateBaseFields(agent.Type); svcErr != nil {
		return nil, svcErr
	}
	// The authz check evaluates the resolved target OU, so resolving it comes first and reports an OU or
	// project that does not exist. The check runs before the rest of the validation, so a caller learns
	// nothing about the agent types of an OU it cannot create agents in.
	if svcErr := s.resolveAgentOU(ctx, agent); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkAgentAccess(ctx, security.ActionCreateAgent, agent.OUID, "", ""); svcErr != nil {
		return nil, svcErr
	}

	clientID, clientSecret, _, svcErr := s.validatePlacedAgent(ctx, agent, "")
	if svcErr != nil {
		return nil, svcErr
	}

	now := time.Now()
	validUntil, svcErr := resolveRequestedValidUntil(agent.ValidUntil, now, isLifespanRestore(ctx))
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
	if agent.Kind == providers.AgentKindPersonalAssistant {
		if svcErr := s.checkPersonalAgentOwner(ctx, owner); svcErr != nil {
			return nil, svcErr
		}
		// A suspended owner's personal agents are suspended with them, so none is created for one.
		hold, svcErr := s.governance.AccessHold(ctx, owner.ID)
		if svcErr != nil {
			s.logger.Error(ctx, "Failed to check the owner's suspension", log.String("errorCode", svcErr.Code))
			return nil, &tidcommon.InternalServerError
		}
		if hold == providers.AccessRefusalReasonSuspended {
			return nil, &ErrorOwnerSuspended
		}
	} else if owner == nil {
		var svcErr *tidcommon.ServiceError
		if owner, svcErr = s.callerAsOwner(ctx); svcErr != nil {
			return nil, svcErr
		}
	} else if svcErr := s.checkOwner(ctx, owner); svcErr != nil {
		return nil, svcErr
	}

	e, sysCredsJSON, buildErr := buildAgentEntity(agentID, agent.Type, agent.OUID, agent.Attributes,
		model.AgentSystemAttributes{
			Description: agent.Description,
			Owner:       owner,
			ClientID:    clientID,
			Kind:        agent.Kind,
		}, clientSecret, validUntil.String())
	if buildErr != nil {
		s.logger.Error(ctx, "Failed to build agent entity", log.Error(buildErr))
		return nil, &tidcommon.InternalServerError
	}
	e.ProjectID = agent.ProjectID

	createdEntity, entErr := s.entityService.CreateEntity(ctx, e, sysCredsJSON, nil)
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
		agent.Type, agent.Description, agent.LogoURL, createdEntity.Attributes,
		authFlowID, regFlowID, agent.IsRegistrationFlowEnabled,
		agent.ThemeID, agent.LayoutID, assertion, loginConsent,
		agent.AllowedUserTypes, agent.AllowedAgentTypes, inboundConfigs)
	resp.OUID = agent.OUID
	resp.ProjectID = agent.ProjectID
	resp.ValidUntil, resp.Lifespan = lifespanView(&validUntil, now)
	resp.Kind = agent.Kind
	s.populateOUHandleForComplete(ctx, resp)
	s.captureValues(ctx, resp, agent.Attributes)
	s.publishLifespanSet(ctx, agentID, validUntil)
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

	// Check authz using the agent's OU ID (fetched from store).
	if svcErr := s.checkAgentAccess(ctx, security.ActionReadAgent, e.OUID, agentID, ""); svcErr != nil {
		return nil, svcErr
	}

	resp, svcErr := s.composeGetResponse(ctx, e)
	if svcErr != nil {
		return nil, svcErr
	}

	if includeDisplay {
		displayPaths := s.resolveDisplayPaths(ctx, []string{e.Type})
		resp.Display = sysutils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayPaths)
		s.populateOUHandleForGet(ctx, resp)
	}

	return resp, nil
}

// GetAgentByClientID returns the agent that holds the given OAuth client ID.
func (s *agentService) GetAgentByClientID(ctx context.Context, clientID string, includeDisplay bool) (
	*model.AgentGetResponse, *tidcommon.ServiceError) {
	if clientID == "" {
		return nil, &ErrorMissingClientID
	}

	agentID, err := s.entityService.IdentifyEntity(ctx, map[string]interface{}{fieldClientID: clientID})
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) || errors.Is(err, entity.ErrAmbiguousEntity) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to find agent by client ID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if agentID == nil || *agentID == "" {
		return nil, &ErrorAgentNotFound
	}

	// GetAgent refuses an entity that is not an agent, such as an application holding the client ID.
	return s.GetAgent(ctx, *agentID, includeDisplay)
}

// GetClientRules returns the runtime rules for an agent's client, or nil when none apply. Only a personal
// agent has rules: it acts only for its owner. It serves the runtime, so it runs no authorization check. A
// personal agent without a local owner is refused as a server error, so no caller can mistake it for an agent
// that any user may sign in to.
func (s *agentService) GetClientRules(ctx context.Context, agentID string) (
	*providers.ClientRules, *tidcommon.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}

	e, err := s.entityService.GetEntityProfile(ctx, agentID)
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

	sysAttrs := readSystemAttributes(e.SystemAttributes)
	if sysAttrs.Kind != providers.AgentKindPersonalAssistant {
		return nil, nil
	}
	if sysAttrs.Owner == nil || !sysAttrs.Owner.Local() {
		s.logger.Error(ctx, "Personal agent has no local owner", log.String("agentID", agentID))
		return nil, &tidcommon.InternalServerError
	}

	return &providers.ClientRules{RequiredSubjectID: sysAttrs.Owner.ID}, nil
}

// GetOwnedAgent returns an agent of any kind that the owner owns as a local owner, as GetAgent returns it. Any other
// agent is not found.
func (s *agentService) GetOwnedAgent(ctx context.Context, ownerID, agentID string,
	includeDisplay bool) (*model.AgentGetResponse, *tidcommon.ServiceError) {
	e, svcErr := s.loadOwnedAgent(ctx, security.ActionReadAgent, ownerID, agentID)
	if svcErr != nil {
		return nil, svcErr
	}
	resp, svcErr := s.composeGetResponse(ctx, e)
	if svcErr != nil {
		return nil, svcErr
	}
	if includeDisplay {
		displayPaths := s.resolveDisplayPaths(ctx, []string{e.Type})
		resp.Display = sysutils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayPaths)
		s.populateOUHandleForGet(ctx, resp)
	}
	return resp, nil
}

// GetOwnedAgents returns a page of the agents the owner owns as a local owner, in the organization unit the
// caller's token names, of the given kind, or of every kind when kind is empty. A token that names no organization
// unit lists none. An external owner with the same ID names someone else, so its agents are left out. It serves the
// owner, so it runs no administrator authorization check; the caller passes the owner.
func (s *agentService) GetOwnedAgents(ctx context.Context, ownerID string, kind providers.AgentKind,
	limit, offset int) (*model.AgentListResponse, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = defaultAgentPageSize
	}
	sysQuery := entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{
		fieldOwnerID: ownerID, fieldOwnerIsLocal: true,
	}}
	var listQuery string
	if kind != "" {
		sysQuery.SystemAttributes[fieldKind] = string(kind)
		listQuery = "&" + kindQueryParam + "=" + url.QueryEscape(string(kind))
	}

	scope := &sysauthz.AccessibleResources{IDs: []string{}}
	if ouID := security.GetOUID(ctx); ouID != "" {
		scope.IDs = []string{ouID}
	}
	resp, svcErr := s.listAgentsBySystemAttributesIn(ctx, sysQuery, scope, limit, offset, false, listQuery)
	if svcErr != nil {
		return nil, svcErr
	}
	resp.Links = sysutils.BuildPaginationLinks(ownedAgentsBasePath, limit, offset, resp.TotalResults, listQuery)
	return resp, nil
}

// UpdateOwnedAgent replaces the part of an agent its owner may change: the description, the schema attributes,
// the grant types and the redirect URIs. The attributes, grant types and redirect URIs are required. The OAuth
// change is validated before anything is written, so a refused grant or redirect URI changes nothing.
func (s *agentService) UpdateOwnedAgent(ctx context.Context, ownerID, agentID string, kind providers.AgentKind,
	req *model.UpdateOwnedAgentRequest) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	updated, svcErr := s.updateOwnedAgent(ctx, ownerID, agentID, kind, req)
	s.publishAgentEvent(ctx, audit.EventAgentUpdate, audit.ActionUpdate, agentID, svcErr)
	return updated, svcErr
}

// updateOwnedAgent applies an owner's update to their agent without publishing its audit event.
func (s *agentService) updateOwnedAgent(ctx context.Context, ownerID, agentID string, kind providers.AgentKind,
	req *model.UpdateOwnedAgentRequest) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if req == nil || len(req.Attributes) == 0 || string(req.Attributes) == "null" || req.GrantTypes == nil ||
		req.RedirectURIs == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	e, svcErr := s.loadOwnedAgent(ctx, security.ActionUpdateAgent, ownerID, agentID)
	if svcErr != nil {
		return nil, svcErr
	}
	sysAttrs := readSystemAttributes(e.SystemAttributes)
	if sysAttrs.Kind != kind {
		return nil, &ErrorAgentNotFound
	}
	if e.IsReadOnly {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	client, oauthProfile, svcErr := s.ownedAgentOAuthUpdate(ctx, agentID, sysAttrs.ClientID, kind, req)
	if svcErr != nil {
		return nil, svcErr
	}

	if err := s.entityService.UpdateAttributes(ctx, agentID, req.Attributes); err != nil {
		if mapped := mapEntityError(err); mapped != nil {
			return nil, mapped
		}
		s.logger.Error(ctx, "Failed to update agent attributes", log.MaskedString("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if err := s.inboundClientService.UpdateInboundClient(
		ctx, client, oauthProfile, false, sysAttrs.ClientID); err != nil {
		return nil, s.inboundUpdateError(ctx, agentID, err)
	}
	sysAttrs.Description = req.Description
	if err := s.entityService.UpdateSystemAttributes(
		ctx, agentID, buildSystemAttributesJSON(sysAttrs, "")); err != nil {
		s.logger.Error(ctx, "Failed to update agent description", log.MaskedString("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	updated, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the updated agent", log.MaskedString("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	full, svcErr := s.composeGetResponse(ctx, updated)
	if svcErr != nil {
		return nil, svcErr
	}
	resp := &model.AgentCompleteResponse{
		ID:                    full.ID,
		OUID:                  full.OUID,
		ProjectID:             full.ProjectID,
		Type:                  full.Type,
		Description:           full.Description,
		LogoURL:               full.LogoURL,
		Owner:                 full.Owner,
		Attributes:            full.Attributes,
		Status:                full.Status,
		LastLoginAt:           full.LastLoginAt,
		Kind:                  full.Kind,
		ValidUntil:            full.ValidUntil,
		Lifespan:              full.Lifespan,
		InboundAuthProfileReq: full.InboundAuthProfileReq,
		InboundAuthConfig:     full.InboundAuthConfig,
	}
	s.populateOUHandleForComplete(ctx, resp)
	s.captureValues(ctx, resp, req.Attributes)
	return resp, nil
}

// ownedAgentOAuthUpdate builds the agent's inbound client and OAuth profile with the requested grant types and
// redirect URIs, and validates them without writing. The grant types go through the same policy as on creation
// (resolveGrantTypes) for the agent's kind; an empty list leaves the agent with no grants, so it gets no tokens.
// The stored key is carried over, since the update writes the certificate from the profile.
func (s *agentService) ownedAgentOAuthUpdate(ctx context.Context, agentID, clientID string, kind providers.AgentKind,
	req *model.UpdateOwnedAgentRequest,
) (*inboundmodel.InboundClient, *providers.OAuthProfile, *tidcommon.ServiceError) {
	storedClient, err := s.inboundClientService.GetInboundClientByEntityID(ctx, agentID)
	if err != nil {
		s.logger.Error(ctx, "Failed to load the agent's inbound client",
			log.MaskedString("agentID", agentID), log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}
	storedProfile, err := s.inboundClientService.GetOAuthProfileByEntityID(ctx, agentID)
	if err != nil {
		s.logger.Error(ctx, "Failed to load the agent's OAuth profile",
			log.MaskedString("agentID", agentID), log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}
	// The stored records may be shared with a cache, so the update works on copies: a refused change must not
	// alter what later reads see.
	client, profile := *storedClient, *storedProfile
	certificate, certErr := s.inboundClientService.GetCertificate(ctx, cert.CertificateReferenceTypeOAuthApp, clientID)
	if certErr != nil {
		return nil, nil, s.translateCertOperationError(ctx, certErr)
	}
	profile.Certificate = certificate

	profile.RedirectURIs = req.RedirectURIs
	profile.GrantTypes, profile.ResponseTypes, profile.PKCERequired = []string{}, nil, false
	if len(req.GrantTypes) > 0 {
		_, storedResponseTypes := convertGrantAndResponseTypes(storedProfile)
		configs, svcErr := resolveGrantTypes([]providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				GrantTypes:              req.GrantTypes,
				ResponseTypes:           storedResponseTypes,
				PKCERequired:            storedProfile.PKCERequired,
				PublicClient:            storedProfile.PublicClient,
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethod(storedProfile.TokenEndpointAuthMethod),
			},
		}}, false, kind)
		if svcErr != nil {
			return nil, nil, svcErr
		}
		resolved := configs[0].OAuthConfig
		profile.GrantTypes = sysutils.ConvertToStringSlice(resolved.GrantTypes)
		profile.ResponseTypes = sysutils.ConvertToStringSlice(resolved.ResponseTypes)
		profile.PKCERequired = resolved.PKCERequired
	}

	if err := s.inboundClientService.Validate(ctx, &client, &profile, false, clientID); err != nil {
		return nil, nil, s.inboundUpdateError(ctx, agentID, err)
	}
	return &client, &profile, nil
}

// inboundUpdateError maps an inbound client validation or update failure to the agent API error.
func (s *agentService) inboundUpdateError(ctx context.Context, agentID string, err error) *tidcommon.ServiceError {
	if svcErr := s.translateInboundClientError(ctx, err); svcErr != nil {
		return svcErr
	}
	s.logger.Error(ctx, "Failed to update the agent's inbound client",
		log.MaskedString("agentID", agentID), log.Error(err))
	return &tidcommon.InternalServerError
}

// DeleteOwnedAgent deletes an agent of the given kind that the owner owns. A declarative agent cannot be deleted.
func (s *agentService) DeleteOwnedAgent(ctx context.Context, ownerID, agentID string,
	kind providers.AgentKind) *tidcommon.ServiceError {
	svcErr := s.deleteOwnedAgent(ctx, ownerID, agentID, kind)
	s.publishAgentEvent(ctx, audit.EventAgentDelete, audit.ActionDelete, agentID, svcErr)
	return svcErr
}

// deleteOwnedAgent deletes an owner's agent without publishing its audit event.
func (s *agentService) deleteOwnedAgent(ctx context.Context, ownerID, agentID string,
	kind providers.AgentKind) *tidcommon.ServiceError {
	e, svcErr := s.loadOwnedAgent(ctx, security.ActionDeleteAgent, ownerID, agentID)
	if svcErr != nil {
		return svcErr
	}
	if readSystemAttributes(e.SystemAttributes).Kind != kind {
		return &ErrorAgentNotFound
	}
	if e.IsReadOnly {
		return &ErrorCannotModifyDeclarativeResource
	}
	return s.removeAgent(ctx, agentID)
}

// loadOwnedAgent loads an agent the owner owns as a local owner, in the organization unit the caller's token names,
// or returns not found, and checks that the caller may perform action on it as its owner. A token that names no
// organization unit reaches no agent.
func (s *agentService) loadOwnedAgent(ctx context.Context, action security.Action, ownerID, agentID string) (
	*providers.Entity, *tidcommon.ServiceError) {
	if ownerID == "" || agentID == "" {
		return nil, &ErrorAgentNotFound
	}
	e, err := s.entityService.GetEntity(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent entity", log.MaskedString("agentID", agentID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	sysAttrs := readSystemAttributes(e.SystemAttributes)
	if e.Category != providers.EntityCategoryAgent || localOwnerID(sysAttrs.Owner) != ownerID ||
		e.OUID == "" || e.OUID != security.GetOUID(ctx) {
		return nil, &ErrorAgentNotFound
	}
	if svcErr := s.checkAgentAccess(ctx, action, e.OUID, agentID, ownerID); svcErr != nil {
		return nil, svcErr
	}
	return e, nil
}

// SuspendOwnedAgent places the owner's suspension on an agent of the given kind that the owner owns, after revoking
// the tokens issued to its client. An agent the owner already suspended is left as it is. The organization's
// suspension, if any, is independent of it.
func (s *agentService) SuspendOwnedAgent(ctx context.Context, ownerID, agentID string,
	kind providers.AgentKind) *tidcommon.ServiceError {
	e, svcErr := s.loadOwnedAgent(ctx, security.ActionUpdateAgent, ownerID, agentID)
	if svcErr != nil {
		return svcErr
	}
	if readSystemAttributes(e.SystemAttributes).Kind != kind {
		return &ErrorAgentNotFound
	}
	_, svcErr = s.suspendByOwner(ctx, []providers.Entity{*e})
	return svcErr
}

// UnsuspendOwnedAgent lifts the owner's suspension of an agent of the given kind that the owner owns. An agent the
// owner has not suspended is left as it is, and the organization's suspension, if any, stays.
func (s *agentService) UnsuspendOwnedAgent(ctx context.Context, ownerID, agentID string,
	kind providers.AgentKind) *tidcommon.ServiceError {
	e, svcErr := s.loadOwnedAgent(ctx, security.ActionUpdateAgent, ownerID, agentID)
	if svcErr != nil {
		return svcErr
	}
	if readSystemAttributes(e.SystemAttributes).Kind != kind {
		return &ErrorAgentNotFound
	}
	svcErr = s.governance.Unsuspend(ownerGovernanceContext(ctx), agentID)
	if svcErr == nil || svcErr.Code == identitygovernance.ErrorNotApplicable.Code {
		return nil
	}
	s.logger.Error(ctx, "Failed to lift the owner's suspension", log.MaskedString("agentID", agentID),
		log.String("errorCode", svcErr.Code))
	return &tidcommon.InternalServerError
}

// SuspendOwnedAgents places the owner's suspension on every agent of the given kind that the owner owns as a local
// owner, in the organization unit the caller's token names, and has not already suspended, after revoking their
// tokens in one batch. It returns how many it suspended. A token that names no organization unit suspends none.
func (s *agentService) SuspendOwnedAgents(ctx context.Context, ownerID string,
	kind providers.AgentKind) (int, *tidcommon.ServiceError) {
	if ownerID == "" {
		return 0, &ErrorAgentNotFound
	}
	ouID := security.GetOUID(ctx)
	if ouID == "" {
		return 0, nil
	}
	entities, err := s.entityService.GetEntityListBySystemAttributesAndOUIDs(ctx, providers.EntityCategoryAgent,
		entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{
			fieldOwnerID: ownerID, fieldOwnerIsLocal: true, fieldKind: string(kind),
		}}, []string{ouID}, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the owner's agents", log.Error(err))
		return 0, &tidcommon.InternalServerError
	}
	return s.suspendByOwner(ctx, entities)
}

// suspendByOwner revokes the tokens issued to each agent not already suspended by its owner, in one batch, then
// places the owner's suspension on it. The agents carry their runtime state. It returns how many it suspended. A
// failed revocation suspends none.
func (s *agentService) suspendByOwner(ctx context.Context, agents []providers.Entity) (int, *tidcommon.ServiceError) {
	reads := s.governance.ReadAccess(ctx, agents)
	cutoff := time.Now().Add(getTokenLeeway()).UTC()
	targets := make([]string, 0, len(agents))
	revocations := make([]revocation.CriteriaRevocation, 0, len(agents))
	for i := range agents {
		if suspendedByOwner(reads[i].Status) {
			continue
		}
		targets = append(targets, agents[i].ID)
		// A personal agent's tokens carry its owner as the subject, so its client is what identifies them.
		if clientID := readSystemAttributes(agents[i].SystemAttributes).ClientID; clientID != "" {
			revocations = append(revocations, revocation.CriteriaRevocation{
				Criterion: revocation.Criterion{Type: revocation.CriterionTypeApplicationKey, Value: clientID},
				Mode:      revocation.ModeBeforeAction,
				Cutoff:    cutoff,
				Reason:    revocation.ReasonIdentitySuspended,
				TTL:       s.revocationTTL(ctx, clientID, suspensionRevocationTTL),
			})
		}
	}
	if len(revocations) > 0 {
		if svcErr := s.revoker.RevokeCriteriaBatch(ctx, revocations); svcErr != nil {
			s.logger.Error(ctx, "Failed to revoke the agents' tokens", log.String("errorCode", svcErr.Code))
			return 0, &tidcommon.InternalServerError
		}
	}

	ownerCtx := ownerGovernanceContext(ctx)
	for i, agentID := range targets {
		if svcErr := s.governance.Suspend(ownerCtx, agentID, ""); svcErr != nil {
			s.logger.Error(ctx, "Failed to place the owner's suspension", log.MaskedString("agentID", agentID),
				log.String("errorCode", svcErr.Code))
			return i, &tidcommon.InternalServerError
		}
	}
	return len(targets), nil
}

// SuspensionDependents names the identities suspended together with a user: their personal agents, declarative ones
// included, since governance holds their state apart from the file. Other resource types have none.
func (s *agentService) SuspensionDependents(ctx context.Context, resourceType, id string) ([]string, error) {
	if resourceType != resourcedependency.ResourceTypeUser {
		return nil, nil
	}
	agents, err := s.entityService.GetEntityListBySystemAttributes(ctx, providers.EntityCategoryAgent,
		entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{
			fieldOwnerID: id, fieldKind: string(providers.AgentKindPersonalAssistant),
		}}, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list the personal agents of the user: %w", err)
	}
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		ids = append(ids, agent.ID)
	}
	return ids, nil
}

// revocationTTL is how long a revocation of the tokens issued to an agent's client must be kept: the longest an
// artifact issued to that client can live, and at least floor. When the client cannot be resolved, floor is kept,
// and zero leaves the revocation service on its default.
func (s *agentService) revocationTTL(ctx context.Context, clientID string, floor time.Duration) time.Duration {
	if s.resolveLifetime == nil {
		return floor
	}
	client, err := s.inboundClientService.GetOAuthClientByClientID(ctx, clientID)
	if err != nil || client == nil {
		s.logger.Warn(ctx, "Failed to resolve the agent's token lifetime for a revocation; keeping the default",
			log.MaskedString("clientID", clientID))
		return floor
	}
	return max(floor, s.resolveLifetime(client))
}

// GetAgentList returns a paginated list of agents. An organization unit filter lists the agents that
// unit owns, followed by the ones shared into it.
func (s *agentService) GetAgentList(ctx context.Context, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (
	*model.AgentListResponse, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = 30
	}

	listQuery := sysutils.FilterQueryParam(filters) + sysutils.DisplayQueryParam(includeDisplay)
	ouID, filters, err := filter.SplitOUIDFilter(filters)
	if err != nil {
		return nil, &ErrorInvalidFilter
	}
	// Other conditions cannot narrow the agents shared into an organization unit yet, so they are
	// refused alongside one rather than applied to the owned agents only.
	if ouID != "" && len(filters) > 0 {
		return nil, &ErrorInvalidFilter
	}

	if ouID != "" {
		return s.listAccessibleAgentsInOU(ctx, ouID, limit, offset, includeDisplay, listQuery)
	}
	if query := systemAttributeQuery(filters); query != nil {
		return s.listAgentsBySystemAttributes(ctx, *query, limit, offset, includeDisplay, listQuery)
	}

	accessible, svcErr := s.getAccessibleOUs(ctx)
	if svcErr != nil {
		return nil, svcErr
	}

	// Unfiltered path: system-level caller — return all agents.
	if accessible.AllAllowed {
		return s.listAllAgents(ctx, limit, offset, filters, includeDisplay, listQuery)
	}

	// Filtered path: return agents belonging to the accessible OUs.
	return s.listAgentsByOUIDs(ctx, accessible.IDs, limit, offset, filters, includeDisplay, listQuery)
}

// GetAgentListByProject returns a paginated list of the agents of a project, narrowed to those an
// organization unit owns when the filter names one. A project that does not exist has none. A caller
// limited to some organization units lists only the project's agents in those units.
func (s *agentService) GetAgentListByProject(ctx context.Context, projectID string, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (*model.AgentListResponse, *tidcommon.ServiceError) {
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = 30
	}

	listQuery := "&" + projectIDQueryParam + "=" + url.QueryEscape(projectID) +
		sysutils.FilterQueryParam(filters) + sysutils.DisplayQueryParam(includeDisplay)
	ouID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return nil, &ErrorInvalidFilter
	}
	empty := s.buildListResponse(ctx, []providers.Entity{}, 0, 0, limit, offset, includeDisplay, listQuery)
	if projectID == "" {
		return empty, nil
	}

	accessible, svcErr := s.getAccessibleOUs(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	var ouIDs []string
	switch {
	case ouID != "" && !accessible.AllAllowed && !slices.Contains(accessible.IDs, ouID):
		return empty, nil
	case ouID != "":
		ouIDs = []string{ouID}
	case !accessible.AllAllowed:
		ouIDs = append([]string{}, accessible.IDs...)
	}

	total, err := s.entityService.GetEntityListCountByProject(ctx, providers.EntityCategoryAgent, projectID, ouIDs)
	if err != nil {
		s.logger.Error(ctx, "Failed to count project agents", log.String("projectID", projectID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	entities, err := s.entityService.GetEntityListByProject(ctx, providers.EntityCategoryAgent, projectID, ouIDs,
		limit, offset)
	if err != nil {
		s.logger.Error(ctx, "Failed to list project agents", log.String("projectID", projectID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return s.buildListResponse(ctx, entities, len(entities), total, limit, offset, includeDisplay, listQuery), nil
}

// GetAgentListByOwner returns a paginated list of the agents the given entity owns, narrowed by the
// attribute filters.
func (s *agentService) GetAgentListByOwner(ctx context.Context, ownerID string, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (*model.AgentListResponse, *tidcommon.ServiceError) {
	if ownerID == "" {
		return nil, &ErrorInvalidFilter
	}
	if svcErr := validatePaginationParams(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	if limit == 0 {
		limit = defaultAgentPageSize
	}

	listQuery := sysutils.FilterQueryParam(filters) + sysutils.DisplayQueryParam(includeDisplay)
	ouID, filters, err := filter.SplitOUIDFilter(filters)
	if err != nil || ouID != "" {
		return nil, &ErrorInvalidFilter
	}
	query := entity.SystemAttributeQuery{
		SystemAttributes: map[string]interface{}{fieldOwnerID: ownerID},
		Attributes:       filters,
	}
	return s.listAgentsBySystemAttributes(ctx, query, limit, offset, includeDisplay, listQuery)
}

// listAgentsBySystemAttributes returns a paginated list of the agents a system attribute query selects, from
// the organization units the caller can list agents from.
func (s *agentService) listAgentsBySystemAttributes(ctx context.Context, query entity.SystemAttributeQuery,
	limit, offset int, includeDisplay bool, listQuery string) (*model.AgentListResponse, *tidcommon.ServiceError) {
	accessible, svcErr := s.getAccessibleOUs(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	return s.listAgentsBySystemAttributesIn(ctx, query, accessible, limit, offset, includeDisplay, listQuery)
}

// listAgentsBySystemAttributesIn returns a paginated list of the agents a system attribute query selects, from the
// given organization units.
func (s *agentService) listAgentsBySystemAttributesIn(ctx context.Context, query entity.SystemAttributeQuery,
	accessible *sysauthz.AccessibleResources, limit, offset int, includeDisplay bool, listQuery string) (
	*model.AgentListResponse, *tidcommon.ServiceError) {
	var totalCount int
	var entities []providers.Entity
	var err error
	switch {
	case accessible.AllAllowed:
		totalCount, err = s.entityService.GetEntityListCountBySystemAttributes(
			ctx, providers.EntityCategoryAgent, query)
		if err == nil {
			entities, err = s.entityService.GetEntityListBySystemAttributes(
				ctx, providers.EntityCategoryAgent, query, limit, offset)
		}
	case len(accessible.IDs) == 0:
		entities = []providers.Entity{}
	default:
		totalCount, err = s.entityService.GetEntityListCountBySystemAttributesAndOUIDs(
			ctx, providers.EntityCategoryAgent, query, accessible.IDs)
		if err == nil {
			entities, err = s.entityService.GetEntityListBySystemAttributesAndOUIDs(
				ctx, providers.EntityCategoryAgent, query, accessible.IDs, limit, offset)
		}
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to list agents by system attributes", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return s.buildListResponse(ctx, entities, len(entities), totalCount, limit, offset, includeDisplay,
		listQuery), nil
}

// getAccessibleOUs resolves the set of organization units the caller is authorized to list agents from.
func (s *agentService) getAccessibleOUs(ctx context.Context) (
	*sysauthz.AccessibleResources, *tidcommon.ServiceError) {
	accessible, svcErr := s.authzService.GetAccessibleResources(
		ctx, security.ActionListAgents, security.ResourceTypeOU)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to resolve accessible resources for listing agents",
			log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}
	return accessible, nil
}

// listAccessibleAgentsInOU lists the agents of an organization unit the caller can list agents from.
// An organization unit the caller cannot list agents from has none to show.
func (s *agentService) listAccessibleAgentsInOU(ctx context.Context, ouID string, limit, offset int,
	includeDisplay bool, listQuery string) (*model.AgentListResponse, *tidcommon.ServiceError) {
	accessible, svcErr := s.getAccessibleOUs(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	if !accessible.AllAllowed && !slices.Contains(accessible.IDs, ouID) {
		return s.buildListResponse(ctx, []providers.Entity{}, 0, 0, limit, offset, includeDisplay,
			listQuery), nil
	}
	return s.listAgentsInOU(ctx, ouID, limit, offset, includeDisplay, listQuery)
}

// listAgentsInOU lists the agents an organization unit owns, followed by the ones shared into it.
func (s *agentService) listAgentsInOU(ctx context.Context, ouID string, limit, offset int,
	includeDisplay bool, listQuery string) (*model.AgentListResponse, *tidcommon.ServiceError) {
	page, err := s.entityService.GetEntityPageInOU(
		ctx, providers.EntityCategoryAgent, ouID, entity.OUEntityScopeAll, limit, offset, nil)
	if err != nil {
		s.logger.Error(ctx, "Failed to list agents in organization unit",
			log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// A shared agent is listed under the organization unit it is shared into, not the one owning it.
	for i := range page.Shared {
		page.Shared[i].OUID = ouID
		page.Shared[i].OUHandle = ""
	}
	return s.buildListResponse(ctx, slices.Concat(page.Owned, page.Shared), len(page.Owned),
		page.TotalResults, limit, offset, includeDisplay, listQuery), nil
}

// listAllAgents retrieves agents without OU filtering.
func (s *agentService) listAllAgents(ctx context.Context, limit, offset int,
	filters map[string]interface{}, includeDisplay bool, listQuery string) (
	*model.AgentListResponse, *tidcommon.ServiceError) {
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

	return s.buildListResponse(ctx, entities, len(entities), totalCount, limit, offset, includeDisplay,
		listQuery), nil
}

// listAgentsByOUIDs retrieves agents scoped to the given organization unit IDs. The OU filter is
// applied at the store layer so that page sizes and total counts remain correct.
func (s *agentService) listAgentsByOUIDs(ctx context.Context, ouIDs []string, limit, offset int,
	filters map[string]interface{}, includeDisplay bool, listQuery string) (
	*model.AgentListResponse, *tidcommon.ServiceError) {
	if len(ouIDs) == 0 {
		return s.buildListResponse(ctx, []providers.Entity{}, 0, 0, limit, offset, includeDisplay,
			listQuery), nil
	}

	totalCount, err := s.entityService.GetEntityListCountByOUIDs(
		ctx, providers.EntityCategoryAgent, ouIDs, filters)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent list count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	entities, err := s.entityService.GetEntityListByOUIDs(
		ctx, providers.EntityCategoryAgent, ouIDs, limit, offset, filters)
	if err != nil {
		s.logger.Error(ctx, "Failed to get agent list", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return s.buildListResponse(ctx, entities, len(entities), totalCount, limit, offset, includeDisplay,
		listQuery), nil
}

// UpdateAgent applies a full-replacement update to the agent.
func (s *agentService) UpdateAgent(ctx context.Context, agentID string,
	req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	updated, svcErr := s.updateAgent(ctx, agentID, req)
	s.publishAgentEvent(ctx, audit.EventAgentUpdate, audit.ActionUpdate, agentID, svcErr)
	return updated, svcErr
}

// updateAgent applies a full-replacement update to the agent without publishing its audit event.
func (s *agentService) updateAgent(ctx context.Context, agentID string,
	req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if agentID == "" {
		return nil, &ErrorMissingAgentID
	}
	if req == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if svcErr := validateBaseFields(req.Type); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateLogoURL(req.LogoURL); svcErr != nil {
		return nil, svcErr
	}
	existing, err := s.entityService.GetEntityProfile(ctx, agentID)
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

	now := time.Now()
	lifespan := newLifespanUpdate(existing.SystemAttributes, req.ValidUntil)

	if existing.IsReadOnly {
		s.refuseLifespanUpdate(ctx, agentID, lifespan, lifespanFailureReadOnly)
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	// fail publishes a requested lifespan change as failed with the update.
	fail := func(svcErr *tidcommon.ServiceError) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
		s.failLifespanUpdate(ctx, agentID, lifespan, svcErr)
		return nil, svcErr
	}

	// Check authz using the existing agent's OU ID.
	if svcErr := s.checkAgentAccess(
		ctx, security.ActionUpdateAgent, existing.OUID, agentID, ""); svcErr != nil {
		return fail(svcErr)
	}

	// The lifespan is checked before anything is written, so a refused change applies no field.
	if svcErr := s.checkLifespanUpdate(ctx, agentID, lifespan, now); svcErr != nil {
		return nil, svcErr
	}

	currentSysAttrs := readSystemAttributes(existing.SystemAttributes)
	currentOwner, currentClientID := currentSysAttrs.Owner, currentSysAttrs.ClientID

	normalizeLoginConsent(req.LoginConsent)

	grantedConfigs, svcErr := resolveGrantTypes(req.InboundAuthConfig, req.Delegated, currentSysAttrs.Kind)
	if svcErr != nil {
		return fail(svcErr)
	}
	req.InboundAuthConfig = grantedConfigs

	var existingOAuthMethod string
	existingOAuth, oauthErr := s.inboundClientService.GetOAuthProfileByEntityID(ctx, agentID)
	if oauthErr != nil && !errors.Is(oauthErr, inboundclient.ErrInboundClientNotFound) {
		s.logger.Error(ctx, "Failed to load existing OAuth profile",
			log.String("agentID", agentID), log.Error(oauthErr))
		return fail(&tidcommon.InternalServerError)
	}
	if existingOAuth != nil {
		existingOAuthMethod = existingOAuth.TokenEndpointAuthMethod
	}

	clientID, clientSecret, svcErr := s.resolveOAuthCredentials(
		ctx, req.InboundAuthConfig, currentClientID, existingOAuthMethod)
	if svcErr != nil {
		return fail(svcErr)
	}

	owner, svcErr := s.resolveUpdateOwner(ctx, req.Owner, currentOwner, currentSysAttrs.Kind)
	if svcErr != nil {
		return fail(svcErr)
	}

	ouID, projectID, svcErr := s.resolveUpdateOUID(ctx, req, existing.OUID, existing.ProjectID)
	if svcErr != nil {
		return fail(svcErr)
	}

	// If the agent is moving to a different OU, require authorization for the destination OU as
	// well. Checked here rather than beside the check above because resolveUpdateOUID resolves
	// ouHandle to an OU ID; both checks still precede every mutation below.
	if ouID != existing.OUID {
		if svcErr := s.checkAgentAccess(
			ctx, security.ActionUpdateAgent, ouID, agentID, ""); svcErr != nil {
			return fail(svcErr)
		}
	}

	if svcErr := s.validateUpdatedPlacement(ctx, existing, req.Type, ouID); svcErr != nil {
		return fail(svcErr)
	}

	resolvedClient, resolvedOAuth, svcErr := s.reconcileInboundForUpdate(
		ctx, agentID, req, clientID, clientSecret)
	if svcErr != nil {
		return fail(svcErr)
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
		ProjectID:  projectID,
		Attributes: req.Attributes,
	}
	// The agent kind is fixed when the agent is created, so the stored value is always kept.
	updatedEntity.SystemAttributes = buildSystemAttributesJSON(model.AgentSystemAttributes{
		Description: req.Description,
		Owner:       owner,
		ClientID:    clientID,
		Kind:        currentSysAttrs.Kind,
	}, "")

	updated, err := s.writeAgentUpdate(ctx, agentID, updatedEntity, lifespan)
	if err != nil {
		if mapped := mapEntityError(err); mapped != nil {
			return fail(mapped)
		}
		s.logger.Error(ctx, "Failed to update agent entity", log.String("agentID", agentID), log.Error(err))
		return fail(&tidcommon.InternalServerError)
	}
	currentValidUntil := s.completeLifespanUpdate(ctx, agentID, lifespan, now)

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
		req.Type, req.Description, req.LogoURL, req.Attributes,
		authFlowID, regFlowID, resolvedClient.IsRegistrationFlowEnabled,
		req.ThemeID, req.LayoutID, assertion, loginConsent,
		req.AllowedUserTypes, req.AllowedAgentTypes, inboundConfigs)
	resp.OUID = ouID
	resp.ProjectID = projectID
	access := s.governance.ReadEntityAccess(ctx, *updated)
	resp.Status = &access.Status
	resp.LastLoginAt = access.LastLoginAt
	resp.ValidUntil, resp.Lifespan = lifespanView(currentValidUntil, now)
	resp.Kind = currentSysAttrs.Kind
	s.populateOUHandleForComplete(ctx, resp)
	s.captureValues(ctx, resp, req.Attributes)
	return resp, nil
}

// UpdateAgentCredentials sets the given credentials of an agent, each a plaintext value of a credential
// the agent's type declares, and keeps the ones not given as they are.
func (s *agentService) UpdateAgentCredentials(ctx context.Context, agentID string,
	credentials json.RawMessage) *tidcommon.ServiceError {
	if agentID == "" {
		return &ErrorMissingAgentID
	}
	var values map[string]string
	if err := json.Unmarshal(credentials, &values); err != nil || len(values) == 0 {
		return &ErrorInvalidRequestFormat
	}
	existing, err := s.entityService.GetEntityProfile(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent entity for a credential update",
			log.MaskedString("agentID", agentID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if existing.Category != providers.EntityCategoryAgent {
		return &ErrorAgentNotFound
	}
	if existing.IsReadOnly {
		return &ErrorCannotModifyDeclarativeResource
	}
	if svcErr := s.checkAgentAccess(
		ctx, security.ActionUpdateAgent, existing.OUID, agentID, ""); svcErr != nil {
		return svcErr
	}

	// The entity service refuses a credential the type does not declare, or an empty one.
	if err = s.entityService.UpdateCredentials(ctx, agentID, credentials); err != nil {
		if mapped := mapEntityError(err); mapped != nil {
			return mapped
		}
		s.logger.Error(ctx, "Failed to update agent credentials", log.MaskedString("agentID", agentID),
			log.Error(err))
		return &tidcommon.InternalServerError
	}
	s.captureCredentials(ctx, existing, values)
	return nil
}

// RegenerateAgentSecret replaces the client secret of an agent that authenticates with one, and returns
// the new secret. That is the only time it can be read, since it is hashed when stored.
func (s *agentService) RegenerateAgentSecret(ctx context.Context, agentID string) (
	string, *tidcommon.ServiceError) {
	secret, svcErr := s.regenerateAgentSecret(ctx, agentID)
	s.publishAgentEvent(ctx, audit.EventAgentUpdate, audit.ActionUpdate, agentID, svcErr)
	return secret, svcErr
}

// regenerateAgentSecret replaces an agent's client secret without publishing its audit event.
func (s *agentService) regenerateAgentSecret(ctx context.Context, agentID string) (
	string, *tidcommon.ServiceError) {
	if agentID == "" {
		return "", &ErrorMissingAgentID
	}

	existing, err := s.entityService.GetEntityProfile(ctx, agentID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return "", &ErrorAgentNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent entity for secret regeneration",
			log.MaskedString("agentID", agentID), log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	if existing.Category != providers.EntityCategoryAgent {
		return "", &ErrorAgentNotFound
	}
	if existing.IsReadOnly {
		return "", &ErrorCannotModifyDeclarativeResource
	}
	if svcErr := s.checkAgentAccess(ctx, security.ActionUpdateAgent, existing.OUID, agentID, ""); svcErr != nil {
		return "", svcErr
	}

	sysAttrs := readSystemAttributes(existing.SystemAttributes)
	if sysAttrs.Kind == providers.AgentKindPersonalAssistant {
		return "", &ErrorPersonalAgentSecretNotRegenerable
	}
	clientID := sysAttrs.ClientID
	oauth, err := s.inboundClientService.GetOAuthProfileByEntityID(ctx, agentID)
	if err != nil && !errors.Is(err, inboundclient.ErrInboundClientNotFound) {
		s.logger.Error(ctx, "Failed to load OAuth profile for secret regeneration",
			log.MaskedString("agentID", agentID), log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	if clientID == "" || !usesClientSecret(oauth) {
		return "", &ErrorClientSecretNotUsed
	}

	clientSecret, err := oauthutils.GenerateOAuth2ClientSecret()
	if err != nil {
		s.logger.Error(ctx, "Failed to generate client secret", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	sysCredsJSON, err := buildSystemCredentialsJSON(clientSecret)
	if err != nil {
		s.logger.Error(ctx, "Failed to build agent system credentials", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	if err = s.entityService.UpdateSystemCredentials(ctx, agentID, sysCredsJSON); err != nil {
		s.logger.Error(ctx, "Failed to update agent system credentials",
			log.MaskedString("agentID", agentID), log.Error(err))
		return "", &tidcommon.InternalServerError
	}

	s.captureValues(ctx, &model.AgentCompleteResponse{
		ID:         agentID,
		OUID:       existing.OUID,
		Type:       existing.Type,
		Attributes: existing.Attributes,
		InboundAuthConfig: annotateOAuthConfig([]providers.InboundAuthConfigWithSecret{{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: oauthProfileToComplete(clientID, oauth),
		}}, clientID, clientSecret),
	}, nil)
	return clientSecret, nil
}

// DeleteAgent removes the agent and its associated inbound client.
// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (s *agentService) SetDependencyRegistry(r resourcedependency.Registry) {
	s.dependencyRegistry = r
}

// DeleteAgent removes the agent and its associated inbound client.
func (s *agentService) DeleteAgent(ctx context.Context, agentID string) *tidcommon.ServiceError {
	svcErr := s.deleteAgent(ctx, agentID)
	s.publishAgentEvent(ctx, audit.EventAgentDelete, audit.ActionDelete, agentID, svcErr)
	return svcErr
}

// deleteAgent removes the agent and its associated inbound client without publishing its audit event.
func (s *agentService) deleteAgent(ctx context.Context, agentID string) *tidcommon.ServiceError {
	if agentID == "" {
		return &ErrorMissingAgentID
	}

	existing, err := s.entityService.GetEntityProfile(ctx, agentID)
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

	// Check authz before the cascade below, which is not transactional with the entity delete.
	if svcErr := s.checkAgentAccess(
		ctx, security.ActionDeleteAgent, existing.OUID, agentID, ""); svcErr != nil {
		return svcErr
	}

	return s.removeAgent(ctx, agentID)
}

// removeAgent removes an agent whose caller is already authorized: its dependents, its inbound client, then
// its entity.
func (s *agentService) removeAgent(ctx context.Context, agentID string) *tidcommon.ServiceError {
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

// publishAgentEvent publishes the audit event of an attempt on the agent with agentID, after the change
// committed or was refused. A delete of a missing agent fails, so it is recorded as failed. An update
// whose entity write committed but whose new client secret could not be stored is recorded as failed,
// and the role assignments and group memberships a delete removes write no events: known limitations.
// Neither the agent's client credentials nor its attributes are recorded.
func (s *agentService) publishAgentEvent(ctx context.Context, name string, action audit.Action,
	agentID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no agent, as forensic
	// evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeAgent, ID: agentID}
	eventbus.TryPublish(ctx, s.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}

// GetResourceDependencies returns the agents that reference the resource identified by
// (resourceType, id). It implements the resourcedependency.Provider interface.
//
// Agents reference a user through their owner attribute and an agent type through their type, both
// of which live on the agent entity rather than in the inbound-client store, so these dependencies
// are resolved separately. All other reference types, and the agent types an agent allows to sign in,
// are resolved via the inbound-client store, which decides what is tracked. An agent type target is
// named by its handle, which is what agents hold, while the dependencies reported for it carry the
// agents' IDs. The number of referencing entities is bounded by MaxCompositeStoreRecords.
func (s *agentService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	switch resourceType {
	case resourcedependency.ResourceTypeUser:
		return s.getAgentsByOwner(ctx, id)
	case resourcedependency.ResourceTypeAgentType:
		return s.getAgentTypeUsages(ctx, id)
	}
	return s.getAgentsByReference(ctx, resourceType, id, resourcedependency.BehaviorFallback)
}

// getAgentTypeUsages returns the agents that block the deletion of an agent type: the agents of that type,
// and the agents that allow it to sign in, since a deleted handle could be recreated in another
// organization unit and inherit the allowance. An agent that does both is listed once.
func (s *agentService) getAgentTypeUsages(
	ctx context.Context, agentType string) ([]resourcedependency.ResourceDependency, error) {
	usages, err := s.getAgentsByType(ctx, agentType)
	if err != nil {
		return nil, err
	}
	allowing, err := s.getAgentsByReference(
		ctx, resourcedependency.ResourceTypeAgentType, agentType, resourcedependency.BehaviorRestrict)
	if err != nil {
		return nil, err
	}

	listed := make(map[string]struct{}, len(usages))
	for _, usage := range usages {
		listed[usage.ID] = struct{}{}
	}
	for _, usage := range allowing {
		if _, ok := listed[usage.ID]; !ok {
			usages = append(usages, usage)
		}
	}
	return usages, nil
}

// getAgentsByReference returns the agents whose inbound client references the resource, reported with
// the given behavior on delete.
func (s *agentService) getAgentsByReference(ctx context.Context, resourceType, id, behavior string) (
	[]resourcedependency.ResourceDependency, error) {
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

	displayPaths := s.resolveDisplayPaths(ctx, entityTypes(entities))
	usages := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, e := range entities {
		// Applications and agents share the inbound-client store; only report agents.
		if e.Category != providers.EntityCategoryAgent {
			continue
		}
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeAgent,
			ID:               e.ID,
			DisplayName:      sysutils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayPaths),
			BehaviorOnDelete: behavior,
		})
	}
	return usages, nil
}

// getAgentsByOwner returns the agents that list the given user as their owner, found through the owner
// in the agents' system attributes. The number of agents returned is bounded by MaxCompositeStoreRecords.
func (s *agentService) getAgentsByOwner(
	ctx context.Context, ownerID string) ([]resourcedependency.ResourceDependency, error) {
	entities, err := s.entityService.GetEntityListBySystemAttributes(ctx, providers.EntityCategoryAgent,
		entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{fieldOwnerID: ownerID}},
		serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		s.logger.Error(ctx, "Failed to list agents", log.Error(err))
		return nil, err
	}

	displayPaths := s.resolveDisplayPaths(ctx, entityTypes(entities))
	usages := make([]resourcedependency.ResourceDependency, 0)
	for _, e := range entities {
		// Only a local owner is an entity of this deployment whose deletion an agent can block.
		if owner := readSystemAttributes(e.SystemAttributes).Owner; owner == nil || !owner.Local() {
			continue
		}
		// An agent cannot exist without its owner, so ownership blocks the owner's deletion, except for a personal
		// agent the owner's deletion removes with them (see CascadeDeleteDependencies).
		behavior := resourcedependency.BehaviorRestrict
		if s.deletedWithOwner(&e) {
			behavior = resourcedependency.BehaviorCascade
		}
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeAgent,
			ID:               e.ID,
			DisplayName:      sysutils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayPaths),
			BehaviorOnDelete: behavior,
		})
	}
	return usages, nil
}

// getAgentsByType returns the agents of the given agent type. An agent cannot exist without its agent
// type, so each blocks the agent type's deletion. The number of agents returned is bounded by
// MaxCompositeStoreRecords.
func (s *agentService) getAgentsByType(
	ctx context.Context, agentType string) ([]resourcedependency.ResourceDependency, error) {
	entities, err := s.entityService.GetEntityListByType(ctx, providers.EntityCategoryAgent, agentType,
		serverconst.MaxCompositeStoreRecords)
	if err != nil {
		// The registry logs provider errors; don't double-log here.
		return nil, err
	}
	if len(entities) == 0 {
		return []resourcedependency.ResourceDependency{}, nil
	}

	displayPaths := s.resolveDisplayPaths(ctx, []string{agentType})
	usages := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, e := range entities {
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeAgent,
			ID:               e.ID,
			DisplayName:      sysutils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayPaths),
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return usages, nil
}

// deletedWithOwner reports whether an agent is deleted together with its owner: a personal agent that can be
// deleted at runtime, where its tokens can be revoked first. A declarative agent can only be removed from its
// file, and without a revoker, which is the control plane, its tokens could not be revoked, so it keeps blocking
// the owner's deletion.
func (s *agentService) deletedWithOwner(e *providers.Entity) bool {
	return s.revoker != nil && !e.IsReadOnly &&
		readSystemAttributes(e.SystemAttributes).Kind == providers.AgentKindPersonalAssistant
}

// CascadeDeleteDependencies implements resourcedependency.CascadeDeleter. It deletes the personal agents of a
// deleted user, after revoking every token issued to their clients; other resource types have no agents to delete.
// It returns how many agents it deleted, and fails on the first error so the user's deletion is aborted.
func (s *agentService) CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error) {
	if resourceType != resourcedependency.ResourceTypeUser {
		return 0, nil
	}
	entities, err := s.entityService.GetEntityListBySystemAttributes(ctx, providers.EntityCategoryAgent,
		entity.SystemAttributeQuery{SystemAttributes: map[string]interface{}{fieldOwnerID: id}},
		serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to list the user's agents: %w", err)
	}

	agentIDs := make([]string, 0, len(entities))
	revocations := make([]revocation.CriteriaRevocation, 0, len(entities))
	for i := range entities {
		if !s.deletedWithOwner(&entities[i]) {
			continue
		}
		agentIDs = append(agentIDs, entities[i].ID)
		if clientID := readSystemAttributes(entities[i].SystemAttributes).ClientID; clientID != "" {
			revocations = append(revocations, revocation.CriteriaRevocation{
				Criterion: revocation.Criterion{Type: revocation.CriterionTypeApplicationKey, Value: clientID},
				Mode:      revocation.ModeAll,
				Reason:    revocation.ReasonUserDeleted,
				TTL:       s.revocationTTL(ctx, clientID, 0),
			})
		}
	}
	if len(revocations) > 0 {
		if svcErr := s.revoker.RevokeCriteriaBatch(ctx, revocations); svcErr != nil {
			return 0, fmt.Errorf("failed to revoke the tokens of the user's agents: %s", svcErr.Code)
		}
	}

	for i, agentID := range agentIDs {
		svcErr := s.removeAgent(ctx, agentID)
		s.publishAgentEvent(ctx, audit.EventAgentDelete, audit.ActionDelete, agentID, svcErr)
		if svcErr != nil {
			return i, fmt.Errorf("failed to delete agent %s: %s", agentID, svcErr.Code)
		}
	}
	return len(agentIDs), nil
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

	existing, err := s.entityService.GetEntityProfile(ctx, agentID)
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

	// Check authz using the agent's OU ID.
	if svcErr := s.checkAgentAccess(
		ctx, security.ActionReadAgent, existing.OUID, agentID, ""); svcErr != nil {
		return nil, svcErr
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

	existing, err := s.entityService.GetEntityProfile(ctx, agentID)
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

	// Check authz using the agent's OU ID.
	if svcErr := s.checkAgentAccess(
		ctx, security.ActionReadAgent, existing.OUID, agentID, ""); svcErr != nil {
		return nil, svcErr
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

	roles, svcErr := s.roleService.GetUserRoles(ctx, agentID, groupIDs, "")
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
func (s *agentService) ValidateAgent(ctx context.Context, agent *providers.Agent, excludeID string) (
	string, string, inboundmodel.InboundClient, *tidcommon.ServiceError) {
	if agent == nil {
		return "", "", inboundmodel.InboundClient{}, &ErrorInvalidRequestFormat
	}
	if svcErr := validateBaseFields(agent.Type); svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	if svcErr := s.resolveAgentOU(ctx, agent); svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	return s.validatePlacedAgent(ctx, agent, excludeID)
}

// validatePlacedAgent validates an Agent whose organization unit is already resolved, as ValidateAgent
// does after resolving it.
func (s *agentService) validatePlacedAgent(ctx context.Context, agent *providers.Agent, excludeID string) (
	string, string, inboundmodel.InboundClient, *tidcommon.ServiceError) {
	if svcErr := validateLogoURL(agent.LogoURL); svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	owner, svcErr := normalizeOwner(agent.Owner)
	if svcErr != nil {
		return "", "", inboundmodel.InboundClient{}, svcErr
	}
	if !agent.Kind.IsValid() {
		return "", "", inboundmodel.InboundClient{}, &ErrorInvalidKind
	}
	if agent.Kind == providers.AgentKindPersonalAssistant {
		if owner == nil {
			return "", "", inboundmodel.InboundClient{}, &ErrorOwnerNotFound
		}
		// A personal agent acts only for its owner, who signs in here, so the owner must be a local user.
		if !owner.Local() {
			return "", "", inboundmodel.InboundClient{}, &ErrorInvalidOwner
		}
		if svcErr := s.applyPersonalAgentSetup(agent); svcErr != nil {
			return "", "", inboundmodel.InboundClient{}, svcErr
		}
	}
	agent.Owner = owner
	if svcErr := s.validateOrganizationUnitForAgentType(ctx, agent.Type, agent.OUID); svcErr != nil {
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

		if inboundclient.UsesClientSecret(oauthCfg) && oauthCfg.ClientSecret == "" {
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
		agent.LoginConsent, agent.AllowedUserTypes, agent.AllowedAgentTypes, agent.SubjectAttribute)

	if needsInboundClient(agent) {
		oauthProfile := buildOAuthProfile(agent.InboundAuthConfig)
		hasSecret := clientSecret != ""
		if err := s.inboundClientService.Validate(ctx, &client, oauthProfile, hasSecret, clientID); err != nil {
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

// checkAgentAccess validates that the caller is authorized to perform the given action on an agent. When ownerID is
// set, the agent's owner is allowed too.
func (s *agentService) checkAgentAccess(
	ctx context.Context, action security.Action, ouID string, resourceID string, ownerID string,
) *tidcommon.ServiceError {
	allowed, svcErr := s.authzService.IsActionAllowed(ctx, action, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeAgent, OUID: ouID, ResourceID: resourceID, ResourceOwnerID: ownerID,
	})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
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

// resolveAgentOU resolves the agent's organization unit in place: from its ouHandle when it names no ID,
// or the system organization unit when it names neither. It checks that a project the agent names holds that
// organization unit, and that the organization unit exists. A resolved agent resolves to the same
// organization unit again.
func (s *agentService) resolveAgentOU(ctx context.Context, agent *providers.Agent) *tidcommon.ServiceError {
	if agent.OUID == "" && agent.OUHandle != "" {
		ou, svcErr := s.ouService.GetOrganizationUnitByPath(ctx, agent.OUHandle)
		if svcErr != nil {
			if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				return &ErrorOrganizationUnitNotFound
			}
			s.logger.Error(ctx, "Failed to resolve OU handle", log.Any("error", svcErr))
			return &tidcommon.InternalServerError
		}
		agent.OUID = ou.ID
	}
	if agent.OUID == "" {
		systemOUID, svcErr := s.ouService.GetSystemOUID(ctx)
		if svcErr != nil {
			return svcErr
		}
		agent.OUID = systemOUID
	}
	// A project checks that the agent's organization unit belongs to it.
	if agent.ProjectID != "" {
		if svcErr := s.validateProjectOU(ctx, agent.ProjectID, agent.OUID); svcErr != nil {
			return svcErr
		}
	}
	return s.validateOUExists(ctx, agent.OUID)
}

// validateUpdatedPlacement checks the agent type placement of an update. Placement is checked only when the
// update changes the organization unit or the type, so an agent placed before the rule existed can still be
// edited in other ways.
func (s *agentService) validateUpdatedPlacement(
	ctx context.Context, existing *providers.Entity, agentType, ouID string,
) *tidcommon.ServiceError {
	if ouID == existing.OUID && agentType == existing.Type {
		return nil
	}
	return s.validateOrganizationUnitForAgentType(ctx, agentType, ouID)
}

// validateOrganizationUnitForAgentType ensures the organization unit is the agent type's organization unit
// or one of its descendants. The agent type is read with the caller's access, and a type the caller cannot read is
// reported the way an unknown type is, so the check reveals nothing about agent types out of the caller's
// reach.
func (s *agentService) validateOrganizationUnitForAgentType(
	ctx context.Context, agentType, ouID string,
) *tidcommon.ServiceError {
	entityType, svcErr := s.entityTypeService.GetEntityTypeByHandle(ctx, entitytype.TypeCategoryAgent, agentType)
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code || svcErr.Code == tidcommon.ErrorUnauthorized.Code {
			return &ErrorAgentTypeNotFound
		}
		s.logger.Error(ctx, "Failed to retrieve agent type", log.String("agentType", agentType),
			log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}

	if entityType.OUID == ouID {
		return nil
	}

	isParent, svcErr := s.ouService.IsParent(ctx, entityType.OUID, ouID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return &ErrorOrganizationUnitNotFound
		}
		s.logger.Error(ctx, "Failed to validate the organization unit hierarchy", log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !isParent {
		return &ErrorOrganizationUnitMismatch
	}
	return nil
}

// checkProjectGiven refuses an agent created without a project where projects are served and one is
// required, and warns about it where one is not required yet.
func (s *agentService) checkProjectGiven(ctx context.Context, projectID string) *tidcommon.ServiceError {
	if projectID != "" || s.projectValidator == nil {
		return nil
	}
	if s.projectRequired {
		return &ErrorProjectRequired
	}
	s.logger.Warn(ctx, "Agent created without a project; a project will be required")
	return nil
}

// validateProjectOU checks that an agent's organization unit belongs to its project. It returns
// ErrorInvalidProject when no such project exists, and ErrorOUNotInProject when the organization unit is
// outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the
// project ID and organization unit as given so that imported configuration retains them.
func (s *agentService) validateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError {
	if s.projectValidator == nil {
		return nil
	}
	svcErr := s.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorOUNotInProject
	default:
		return &tidcommon.InternalServerError
	}
}

// resolveUpdateOUID picks the organization unit and project for an update: the requested ones, or the
// existing ones the request leaves out. The organization unit must belong to the project.
func (s *agentService) resolveUpdateOUID(
	ctx context.Context, req *model.UpdateAgentRequest, existingOUID, existingProjectID string,
) (string, string, *tidcommon.ServiceError) {
	if req.OUID == "" && req.OUHandle != "" {
		ou, ouSvcErr := s.ouService.GetOrganizationUnitByPath(ctx, req.OUHandle)
		if ouSvcErr != nil {
			if ouSvcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				return "", "", &ErrorOrganizationUnitNotFound
			}
			s.logger.Error(ctx, "Failed to resolve OU handle", log.Any("error", ouSvcErr))
			return "", "", &tidcommon.InternalServerError
		}
		req.OUID = ou.ID
	}
	ouID := req.OUID
	if ouID == "" {
		ouID = existingOUID
	} else if ouID != existingOUID {
		if svcErr := s.validateOUExists(ctx, ouID); svcErr != nil {
			return "", "", svcErr
		}
	}
	// A project checks that the agent stays in it; it never moves the agent on its own. An update that
	// names no project keeps the agent in its project.
	projectID := req.ProjectID
	if projectID == "" {
		projectID = existingProjectID
	}
	if projectID == "" {
		return ouID, "", nil
	}
	if svcErr := s.validateProjectOU(ctx, projectID, ouID); svcErr != nil {
		return "", "", svcErr
	}
	return ouID, projectID, nil
}

// resolveUpdateOwner picks the effective owner for an update — either the requested owner or the
// existing one — and checks that a local owner exists when the owner is changing. A personal agent's new
// owner must be a local user.
func (s *agentService) resolveUpdateOwner(
	ctx context.Context, requestedOwner, currentOwner *providers.AgentOwner, kind providers.AgentKind,
) (*providers.AgentOwner, *tidcommon.ServiceError) {
	if requestedOwner == nil {
		return currentOwner, nil
	}
	owner, svcErr := normalizeOwner(requestedOwner)
	if svcErr != nil {
		return nil, svcErr
	}
	if sameOwner(owner, currentOwner) {
		return owner, nil
	}
	if kind == providers.AgentKindPersonalAssistant {
		svcErr = s.checkPersonalAgentOwner(ctx, owner)
	} else {
		svcErr = s.checkOwner(ctx, owner)
	}
	if svcErr != nil {
		return nil, svcErr
	}
	return owner, nil
}

// checkOwner checks that a local owner exists in this deployment. An external owner is managed in another
// identity system, so it is stored as given and never looked up.
func (s *agentService) checkOwner(ctx context.Context, owner *providers.AgentOwner) *tidcommon.ServiceError {
	if !owner.Local() {
		return nil
	}
	_, svcErr := s.validateOwnerExists(ctx, owner.ID)
	return svcErr
}

// normalizeOwner checks an agent owner and returns a copy that states its ID type and whether it is
// local. A nil owner stays nil.
func normalizeOwner(owner *providers.AgentOwner) (*providers.AgentOwner, *tidcommon.ServiceError) {
	if owner == nil {
		return nil, nil
	}
	id := strings.TrimSpace(owner.ID)
	idType := owner.IDType
	if idType == "" {
		idType = providers.AgentOwnerIDTypeUUID
	}
	local := owner.Local()
	if id == "" || utf8.RuneCountInString(id) > maxOwnerIDLength || !slices.Contains(ownerIDTypes, idType) ||
		(local && idType != providers.AgentOwnerIDTypeUUID) || !ownerIDMatchesType(id, idType) {
		return nil, &ErrorInvalidOwner
	}
	return &providers.AgentOwner{ID: id, IDType: idType, IsLocal: &local}, nil
}

// ownerIDMatchesType reports whether an owner ID has the form its ID type needs. An email is a bare
// address and a mobile number is in E.164 form. A uuid is not checked, since an external owner's
// subject is not always a UUID, and a local one is looked up.
func ownerIDMatchesType(id string, idType providers.AgentOwnerIDType) bool {
	switch idType {
	case providers.AgentOwnerIDTypeEmail:
		address, err := mail.ParseAddress(id)
		return err == nil && address.Address == id
	case providers.AgentOwnerIDTypeMobile:
		return ownerMobilePattern.MatchString(id)
	default:
		return true
	}
}

// callerAsOwner returns the authenticated caller as an owner, or nil when there is no caller or the
// caller is the management API key, which is not a person. A caller holding a token this deployment
// did not issue, such as one from the trusted issuer, is external without a lookup. A self-issued
// token is not enough to make the caller local, since it can carry a subject from elsewhere (a JWT
// bearer grant) or one since deleted, so its subject is local only when it is an entity of this
// deployment.
func (s *agentService) callerAsOwner(ctx context.Context) (*providers.AgentOwner, *tidcommon.ServiceError) {
	subject := security.GetSubject(ctx)
	if subject == "" || security.IsManagementAPIKey(ctx) {
		return nil, nil
	}
	local := security.IsSelfIssued(ctx)
	if local {
		if _, svcErr := s.validateOwnerExists(ctx, subject); svcErr != nil {
			if svcErr.Code != ErrorOwnerNotFound.Code {
				return nil, svcErr
			}
			local = false
		}
	}
	return &providers.AgentOwner{ID: subject, IDType: providers.AgentOwnerIDTypeUUID, IsLocal: &local}, nil
}

// checkPersonalAgentOwner checks that a personal agent's owner is a local user, the only owner who can sign in
// for it.
func (s *agentService) checkPersonalAgentOwner(
	ctx context.Context, owner *providers.AgentOwner) *tidcommon.ServiceError {
	if owner == nil || !owner.Local() {
		return &ErrorInvalidOwner
	}
	ownerEntity, svcErr := s.validateOwnerExists(ctx, owner.ID)
	if svcErr != nil {
		return svcErr
	}
	if ownerEntity.Category != providers.EntityCategoryUser {
		return &ErrorOwnerNotFound
	}
	return nil
}

// localOwnerID returns the ID of a local owner, or an empty string when the owner is missing or not local.
func localOwnerID(owner *providers.AgentOwner) string {
	if owner == nil || !owner.Local() {
		return ""
	}
	return owner.ID
}

// sameOwner reports whether two owners name the same owner.
func sameOwner(a, b *providers.AgentOwner) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ID == b.ID && a.IDType == b.IDType && a.Local() == b.Local()
}

// ownerGovernanceContext marks a governance call as the owner's. It runs with the runtime context, since
// governance authorizes administrators and the owner's own checks have already run.
func ownerGovernanceContext(ctx context.Context) context.Context {
	return identitygovernance.WithOwnerApplier(security.WithRuntimeContext(ctx))
}

// suspendedByOwner reports whether a status carries the owner's suspension.
func suspendedByOwner(status providers.AccessStatusReport) bool {
	return status.Details != nil && slices.Contains(status.Details.AppliedBy, providers.SuspensionAppliedByOwner)
}

// validateOwnerExists returns the owner entity, or an error when the given owner ID does not resolve to an
// existing entity. An empty owner ID returns no entity and no error.
func (s *agentService) validateOwnerExists(ctx context.Context, ownerID string) (
	*providers.Entity, *tidcommon.ServiceError) {
	if ownerID == "" {
		return nil, nil
	}
	owner, err := s.entityService.GetEntityProfile(ctx, ownerID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorOwnerNotFound
		}
		s.logger.Error(ctx, "Failed to verify owner existence",
			log.String("ownerID", ownerID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return owner, nil
}

// applyPersonalAgentSetup completes a personal agent's configuration the same way whatever path creates it. The
// login flow falls back to the shipped personal agent login flow, and consent is asked on every
// authorization unless the caller says otherwise. The OAuth client, added when the agent has none, follows the
// personal grant rules (resolvePersonalAssistantGrantTypes); it authenticates with private_key_jwt when it has
// a usable key and with a client secret otherwise. The declarative path reaches it only through ValidateAgent,
// so the grant rules are applied here too.
func (s *agentService) applyPersonalAgentSetup(agent *providers.Agent) *tidcommon.ServiceError {
	if agent.AuthFlowID == "" && agent.AuthFlowHandle == "" {
		agent.AuthFlowHandle = personalAgentAuthFlowHandle
	}
	if agent.LoginConsent == nil {
		agent.LoginConsent = &providers.LoginConsentConfig{}
	}
	if agent.LoginConsent.AlwaysPrompt == nil {
		alwaysPrompt := true
		agent.LoginConsent.AlwaysPrompt = &alwaysPrompt
	}

	if _, svcErr := pickOAuthConfig(agent.InboundAuthConfig); svcErr != nil {
		return svcErr
	}
	if svcErr := applyGrantTypes(agent); svcErr != nil {
		return svcErr
	}
	oauthCfg := agent.InboundAuthConfig[oauthConfigIndex(agent.InboundAuthConfig)].OAuthConfig
	oauthCfg.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodClientSecretBasic
	if oauthCfg.Certificate != nil {
		if svcErr := validateAgentKey(oauthCfg.Certificate); svcErr != nil {
			return svcErr
		}
		oauthCfg.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
	}
	return nil
}

// validateAgentKey checks that an agent's key is usable: a public key is a JWKS document with at least one
// key, and a JWKS URI is an HTTPS URL that does not point to a loopback, link-local or private IP address.
// It is applied to personal agents, whose key anyone registering one can set.
func validateAgentKey(certificate *providers.Certificate) *tidcommon.ServiceError {
	switch certificate.Type {
	case providers.CertificateTypeJWKSURI:
		if syshttp.IsSSRFSafeURL(certificate.Value) != nil {
			return &ErrorInvalidJWKSURI
		}
	case providers.CertificateTypeJWKS:
		var jwks struct {
			Keys []map[string]interface{} `json:"keys"`
		}
		if err := json.Unmarshal([]byte(certificate.Value), &jwks); err != nil || len(jwks.Keys) == 0 {
			return &ErrorInvalidPublicKey
		}
	}
	return nil
}

// personalAgentGrantTypes are the only grant types a personal agent may hold. Each one needs the owner
// to sign in, or refreshes a token issued that way, so the agent can never act on its own authority.
var personalAgentGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeCIBA,
	providers.GrantTypeRefreshToken,
}

// validatePersonalAgentGrantTypes checks that every grant type of a personal agent is in the personal set.
func validatePersonalAgentGrantTypes(grantTypes []providers.GrantType) *tidcommon.ServiceError {
	for _, grantType := range grantTypes {
		if !slices.Contains(personalAgentGrantTypes, grantType) {
			return &ErrorGrantTypeNotAllowedForPersonalAgent
		}
	}
	return nil
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
	requiresSecret := inboundclient.UsesClientSecret(oauthCfg)
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
	agent *providers.Agent, clientSecret string) (
	inboundmodel.InboundClient, *providers.OAuthProfile, *tidcommon.ServiceError) {
	client := buildInboundClientRecord(agentID, agent.AuthFlowID, agent.RegistrationFlowID,
		agent.IsRegistrationFlowEnabled, agent.ThemeID, agent.LayoutID, agent.Assertion,
		agent.LoginConsent, agent.AllowedUserTypes, agent.AllowedAgentTypes, agent.SubjectAttribute)
	setLogoProperty(&client, agent.LogoURL)

	seedClientSubTypeAttribute(agent.InboundAuthConfig)
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

	// Resolve flow handles to IDs when the direct IDs are absent, same as ValidateAgent does for
	// create - a declarative update (e.g. a re-import) that only sets authFlowHandle must not wipe
	// out the previously-resolved AuthFlowID.
	profile := providers.InboundAuthProfile{
		AuthFlowID:             req.AuthFlowID,
		AuthFlowHandle:         req.AuthFlowHandle,
		RegistrationFlowID:     req.RegistrationFlowID,
		RegistrationFlowHandle: req.RegistrationFlowHandle,
	}
	if err := s.inboundClientService.ResolveInboundAuthProfileHandles(ctx, &profile); err != nil {
		if svcErr := translateInboundClientFKError(err); svcErr != nil {
			return inboundmodel.InboundClient{}, nil, svcErr
		}
		s.logger.Error(ctx, "Failed to resolve inbound auth profile handles",
			log.Error(err), log.String("agentID", agentID))
		return inboundmodel.InboundClient{}, nil, &tidcommon.InternalServerError
	}

	client := buildInboundClientRecord(agentID, profile.AuthFlowID, profile.RegistrationFlowID,
		req.IsRegistrationFlowEnabled, req.ThemeID, req.LayoutID, req.Assertion,
		req.LoginConsent, req.AllowedUserTypes, req.AllowedAgentTypes, nil)
	setLogoProperty(&client, req.LogoURL)
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
	sysAttrs := readSystemAttributes(e.SystemAttributes)
	clientID := sysAttrs.ClientID

	access := s.governance.ReadEntityAccess(ctx, *e)
	resp := &model.AgentGetResponse{
		ID:          e.ID,
		OUID:        e.OUID,
		OUHandle:    e.OUHandle,
		ProjectID:   e.ProjectID,
		Type:        e.Type,
		Description: sysAttrs.Description,
		Owner:       sysAttrs.Owner,
		ClientID:    clientID,
		Kind:        sysAttrs.Kind,
		Attributes:  e.Attributes,
		Status:      &access.Status,
		LastLoginAt: access.LastLoginAt,
	}
	resp.ValidUntil, resp.Lifespan = lifespanView(storedLifespanEnd(e.SystemAttributes), time.Now())

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
	resp.AllowedAgentTypes = inbound.AllowedAgentTypes
	resp.LogoURL = logoURLFromProperties(inbound.Properties)

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
// The first ownedCount entities are owned by the listed organization unit and the rest are shared into it.
func (s *agentService) buildListResponse(ctx context.Context, entities []providers.Entity,
	ownedCount, totalCount, limit, offset int, includeDisplay bool, listQuery string) *model.AgentListResponse {
	logoByID := s.agentLogoMap(ctx, entities)
	// One read for the page, so the policy resolves once.
	reads := s.governance.ReadAccess(ctx, entities)
	agents := make([]model.BasicAgentResponse, 0, len(entities))
	now := time.Now()
	for i := range entities {
		e := &entities[i]
		sysAttrs := readSystemAttributes(e.SystemAttributes)
		owned := i < ownedCount
		validUntil, lifespan := lifespanView(storedLifespanEnd(e.SystemAttributes), now)
		agents = append(agents, model.BasicAgentResponse{
			ID:          e.ID,
			OUID:        e.OUID,
			OUHandle:    e.OUHandle,
			ProjectID:   e.ProjectID,
			Type:        e.Type,
			Description: sysAttrs.Description,
			LogoURL:     logoByID[e.ID],
			ClientID:    sysAttrs.ClientID,
			Owner:       sysAttrs.Owner,
			Kind:        sysAttrs.Kind,
			Attributes:  e.Attributes,
			ValidUntil:  validUntil,
			Lifespan:    lifespan,
			// The organization unit an agent is shared into cannot change it until overlay rules are
			// supported for agents.
			IsReadOnly:  e.IsReadOnly || !owned,
			Status:      &reads[i].Status,
			LastLoginAt: reads[i].LastLoginAt,
			IsOwnedByOU: new(owned),
		})
	}

	if includeDisplay {
		displayPaths := s.resolveDisplayPaths(ctx, entityTypes(entities))
		for i := range agents {
			agents[i].Display = sysutils.ResolveDisplay(
				agents[i].ID, agents[i].Type, agents[i].Attributes, displayPaths)
		}
		s.populateOUHandlesForList(ctx, agents)
	}

	return &model.AgentListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(agents),
		Agents:       agents,
		Links:        sysutils.BuildPaginationLinks(agentBasePath, limit, offset, totalCount, listQuery),
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
func needsInboundClient(agent *providers.Agent) bool {
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
		len(agent.AllowedAgentTypes) > 0 ||
		len(agent.InboundAuthConfig) > 0
}

// updateNeedsInboundClient reports whether an update request contains any inbound auth field requiring a client row.
func updateNeedsInboundClient(req *model.UpdateAgentRequest) bool {
	if req == nil {
		return false
	}
	return req.AuthFlowID != "" ||
		req.AuthFlowHandle != "" ||
		req.RegistrationFlowID != "" ||
		req.RegistrationFlowHandle != "" ||
		req.IsRegistrationFlowEnabled ||
		req.ThemeID != "" ||
		req.LayoutID != "" ||
		req.Assertion != nil ||
		req.LoginConsent != nil ||
		len(req.AllowedUserTypes) > 0 ||
		len(req.AllowedAgentTypes) > 0 ||
		len(req.InboundAuthConfig) > 0
}

// validateLogoURL validates the optional logo URL; an empty value is allowed.
func validateLogoURL(logoURL string) *tidcommon.ServiceError {
	if logoURL != "" && !sysutils.IsValidLogoURI(logoURL) {
		return &ErrorInvalidLogoURL
	}
	return nil
}

// validateBaseFields validates the mandatory top-level fields required for both create and update.
func validateBaseFields(agentType string) *tidcommon.ServiceError {
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

// usesClientSecret reports whether a stored OAuth profile authenticates its client with a secret.
func usesClientSecret(p *providers.OAuthProfile) bool {
	if p == nil || p.PublicClient {
		return false
	}
	switch providers.TokenEndpointAuthMethod(p.TokenEndpointAuthMethod) {
	case providers.TokenEndpointAuthMethodClientSecretBasic, providers.TokenEndpointAuthMethodClientSecretPost:
		return true
	case providers.TokenEndpointAuthMethodPrivateKeyJWT, providers.TokenEndpointAuthMethodNone:
		return false
	}
	return false
}

// buildAgentEntity constructs the entity row and system credentials JSON for a new or updated agent.
// validUntil is the stored lifespan end; empty leaves it out, for writes that carry the stored value.
func buildAgentEntity(agentID, agentType, ouID string, attributes json.RawMessage,
	sysAttrs model.AgentSystemAttributes, clientSecret, validUntil string) (*providers.Entity, json.RawMessage, error) {
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
		SystemAttributes: buildSystemAttributesJSON(sysAttrs, validUntil),
	}
	return e, sysCredsJSON, nil
}

// buildSystemAttributesJSON serializes the agent system attributes, and validUntil, into the systemAttributes
// blob. Marshaling strings and an owner cannot fail.
func buildSystemAttributesJSON(sysAttrs model.AgentSystemAttributes, validUntil string) json.RawMessage {
	attrs := map[string]interface{}{}
	if sysAttrs.Description != "" {
		attrs[fieldDescription] = sysAttrs.Description
	}
	if sysAttrs.Owner != nil {
		attrs[fieldOwner] = sysAttrs.Owner
	}
	if sysAttrs.ClientID != "" {
		attrs[fieldClientID] = sysAttrs.ClientID
	}
	if sysAttrs.Kind != "" {
		attrs[fieldKind] = string(sysAttrs.Kind)
	}
	if validUntil != "" {
		attrs[authnprovidercm.SystemAttrValidUntil] = validUntil
	}
	if len(attrs) == 0 {
		return nil
	}
	raw, _ := json.Marshal(attrs)
	return raw
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

// readSystemAttributes deserializes the systemAttributes JSON blob back into the agent system attributes.
func readSystemAttributes(raw json.RawMessage) model.AgentSystemAttributes {
	var sysAttrs model.AgentSystemAttributes
	if len(raw) == 0 {
		return sysAttrs
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return sysAttrs
	}
	if v, ok := attrs[fieldDescription].(string); ok {
		sysAttrs.Description = v
	}
	// A legacy string owner does not decode, and reads as no owner.
	var stored struct {
		Owner *providers.AgentOwner `json:"owner"`
	}
	if json.Unmarshal(raw, &stored) == nil && stored.Owner != nil && stored.Owner.ID != "" {
		sysAttrs.Owner = stored.Owner
	}
	if v, ok := attrs[fieldClientID].(string); ok {
		sysAttrs.ClientID = v
	}
	if v, ok := attrs[fieldKind].(string); ok {
		sysAttrs.Kind = providers.AgentKind(v)
	}
	return sysAttrs
}

// systemAttributeQuery returns the lookup for a filter on the kind or the owner, which are matched against the
// agents' system attributes, the owner by its ID, with the rest of the filter on the schema attributes. It
// returns nil for a filter on schema attributes only.
func systemAttributeQuery(filters map[string]interface{}) *entity.SystemAttributeQuery {
	sysAttrs := map[string]interface{}{}
	attrs := map[string]interface{}{}
	for key, value := range filters {
		switch key {
		case fieldKind:
			sysAttrs[fieldKind] = value
		case fieldOwner:
			sysAttrs[fieldOwnerID] = value
		default:
			attrs[key] = value
		}
	}
	if len(sysAttrs) == 0 {
		return nil
	}
	return &entity.SystemAttributeQuery{SystemAttributes: sysAttrs, Attributes: attrs}
}

// resolveDisplayPaths resolves the display attribute path of each given agent type.
func (s *agentService) resolveDisplayPaths(ctx context.Context, agentTypes []string) map[string]string {
	return entitytype.ResolveDisplayAttributePaths(
		ctx, entitytype.TypeCategoryAgent, agentTypes, s.entityTypeService, s.logger)
}

// entityTypes returns the type name of each entity.
func entityTypes(entities []providers.Entity) []string {
	types := make([]string, 0, len(entities))
	for i := range entities {
		types = append(types, entities[i].Type)
	}
	return types
}

// buildInboundClientRecord constructs an InboundClient record from the agent's identity and inbound auth fields.
func buildInboundClientRecord(agentID, authFlowID, regFlowID string, isRegEnabled bool,
	themeID, layoutID string, assertion *inboundmodel.AssertionConfig,
	loginConsent *inboundmodel.LoginConsentConfig, allowedUserTypes, allowedAgentTypes []string,
	subjectAttribute map[string]string) inboundmodel.InboundClient {
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
		AllowedAgentTypes:         allowedAgentTypes,
		SubjectAttribute:          subjectAttribute,
	}
}

// setLogoProperty stores the logo URL in the inbound client's PROPERTIES. An empty value is left unset.
func setLogoProperty(client *inboundmodel.InboundClient, logoURL string) {
	if logoURL == "" {
		return
	}
	if client.Properties == nil {
		client.Properties = map[string]interface{}{}
	}
	client.Properties[propLogoURL] = logoURL
}

// logoURLFromProperties reads the logo URL from an inbound client's PROPERTIES blob.
func logoURLFromProperties(props map[string]interface{}) string {
	if props == nil {
		return ""
	}
	if logoURL, ok := props[propLogoURL].(string); ok {
		return logoURL
	}
	return ""
}

// agentLogoMap builds a map of entity ID to logo URL for the given agents, looking up only
// the current page's inbound clients.
func (s *agentService) agentLogoMap(ctx context.Context, entities []providers.Entity) map[string]string {
	logoByID := make(map[string]string, len(entities))
	for i := range entities {
		id := entities[i].ID
		inbound, err := s.inboundClientService.GetInboundClientByEntityID(ctx, id)
		if err != nil {
			if !errors.Is(err, inboundclient.ErrInboundClientNotFound) {
				s.logger.Error(ctx, "Failed to load inbound client for agent logo",
					log.String("agentID", id), log.Error(err))
			}
			continue
		}
		if inbound == nil {
			continue
		}
		if logoURL := logoURLFromProperties(inbound.Properties); logoURL != "" {
			logoByID[id] = logoURL
		}
	}
	return logoByID
}

// seedClientSubTypeAttribute selects the sub_type claim for a new agent, so its own tokens are
// distinguishable from an M2M application's. Empty grant types default to client_credentials (see
// buildOAuthProfile), so those are seeded too.
func seedClientSubTypeAttribute(configs []providers.InboundAuthConfigWithSecret) {
	cfg, err := pickOAuthConfig(configs)
	if err != nil || cfg == nil {
		return
	}
	if len(cfg.GrantTypes) > 0 &&
		!slices.Contains(cfg.GrantTypes, providers.GrantTypeClientCredentials) {
		return
	}
	cfg.Token = oauthutils.EnsureClientSubTypeAttribute(cfg.Token)
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
		BackchannelLogoutURI:               cfg.BackchannelLogoutURI,
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
		BackchannelLogoutURI:               p.BackchannelLogoutURI,
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
func buildCompleteResponse(agentID string, owner *providers.AgentOwner,
	clientID, clientSecret, agentType, description, logoURL string,
	attributes json.RawMessage, authFlowID, regFlowID string, isRegEnabled bool,
	themeID, layoutID string, assertion *inboundmodel.AssertionConfig,
	loginConsent *inboundmodel.LoginConsentConfig, allowedUserTypes, allowedAgentTypes []string,
	inboundAuthConfig []providers.InboundAuthConfigWithSecret,
) *model.AgentCompleteResponse {
	resp := &model.AgentCompleteResponse{
		ID:          agentID,
		Type:        agentType,
		Description: description,
		LogoURL:     logoURL,
		Owner:       owner,
		Attributes:  attributes,
		InboundAuthProfileReq: inboundmodel.InboundAuthProfileReq{
			AuthFlowID:                authFlowID,
			RegistrationFlowID:        regFlowID,
			IsRegistrationFlowEnabled: isRegEnabled,
			ThemeID:                   themeID,
			LayoutID:                  layoutID,
			Assertion:                 assertion,
			LoginConsent:              loginConsent,
			AllowedUserTypes:          allowedUserTypes,
			AllowedAgentTypes:         allowedAgentTypes,
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
	case errors.Is(err, inboundclient.ErrOAuthInvalidBackchannelLogoutURI):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.invalid_backchannel_logout_uri_description",
			DefaultValue: "Backchannel logout URI must be an absolute http or https URL without userinfo, " +
				"a fragment or a wildcard",
		})
	case errors.Is(err, inboundclient.ErrOAuthBackchannelLogoutURIRequiresHTTPS):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.backchannel_logout_uri_requires_https_description",
			DefaultValue: "Backchannel logout URI must use https for a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthBackchannelLogoutURIPrivateHost):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.backchannel_logout_uri_private_host_description",
			DefaultValue: "Backchannel logout URI must not name a loopback, link-local or private address",
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
	case errors.Is(err, inboundclient.ErrOAuthRefreshTokenRequiresTokenIssuingGrant):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.refresh_token_requires_token_issuing_grant_description",
			DefaultValue: "refresh_token grant type requires a token-issuing grant type",
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
	case errors.Is(err, inboundclient.ErrOAuthClientIDJAGRequiresTokenExchange):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.idjag_requires_token_exchange_description",
			DefaultValue: "ID-JAG configuration requires the " +
				"urn:ietf:params:oauth:grant-type:token-exchange grant type",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientIDJAGRequiresAllowedAudience):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.idjag_requires_allowed_audience_description",
			DefaultValue: "ID-JAG configuration requires at least one allowed audience, and allowed " +
				"audiences must not be blank",
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
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthRequiresPublicClient):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.none_auth_method_requires_public_client_description",
			DefaultValue: "'none' authentication method requires the client to be a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthCannotHaveSecret):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.none_auth_method_cannot_have_secret_description",
			DefaultValue: "'none' authentication method cannot have a client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.client_credentials_cannot_use_none_auth_description",
			DefaultValue: "client_credentials grant type cannot use 'none' authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientJWTBearerCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.jwt_bearer_cannot_use_none_auth_description",
			DefaultValue: "jwt-bearer grant type requires a confidential client and cannot use 'none' " +
				"token endpoint authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientIDJAGCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.idjag_cannot_use_none_auth_description",
			DefaultValue: "ID-JAG configuration requires a confidential client and cannot use 'none' " +
				"token endpoint authentication method",
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
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWERequiresEncryption):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.userinfo_jwe_requires_encryption_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required when userinfo responseType is JWE",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.agentservice.userinfo_nested_jwt_requires_all_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required " +
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
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedSigningAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.agentservice.idtoken_unsupported_signing_alg_description",
			DefaultValue: "idToken signing algorithm is not supported",
		})
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
	case errors.Is(err, inboundclient.ErrFKInvalidAgentType):
		return &ErrorInvalidAllowedAgentType
	case errors.Is(err, inboundclient.ErrFKAnyAgentTypeNotAlone):
		return &ErrorAnyAgentTypeNotAlone
	case errors.Is(err, inboundclient.ErrUserSchemaLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrAgentSchemaLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrUniqueAttributeLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrFKInvalidSubjectAttributeMapping):
		return &ErrorInvalidSubjectAttributeMapping
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
