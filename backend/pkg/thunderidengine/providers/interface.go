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

// Package providers provides interfaces for the providers module.
package providers

import (
	"context"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AuthnProviderManagerInterface defines the interface for the authentication provider manager.
type AuthnProviderManagerInterface interface {
	AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
		requestedAttributes *RequestedAttributes,
		metadata *AuthnMetadata,
		authUser AuthUser) (AuthUser, AuthenticatedClaims, *common.ServiceError)
	GetEntityReference(ctx context.Context, authUser AuthUser) (
		AuthUser, *EntityReference, *common.ServiceError)
	GetUserAvailableAttributes(ctx context.Context,
		authUser AuthUser) (*AttributesResponse, *common.ServiceError)
	GetUserAttributes(ctx context.Context,
		requestedAttributes *RequestedAttributes,
		metadata *GetAttributesMetadata,
		authUser AuthUser) (AuthUser, *AttributesResponse, *common.ServiceError)
}

// ActorProviderInterface resolves inbound actors and exposes their OAuth and membership data.
type ActorProviderInterface interface {
	GetOAuthClientByClientID(
		ctx context.Context, clientID string,
	) (*OAuthClient, *common.ServiceError)
	GetOAuthProfileByID(
		ctx context.Context, id string,
	) (*OAuthProfile, *common.ServiceError)
	GetInboundClientByID(
		ctx context.Context, id string,
	) (*InboundClient, *common.ServiceError)
	GetFlowInitiationMode(
		ctx context.Context, id string,
	) (FlowInitiationMode, *common.ServiceError)
	AuthenticateActor(
		ctx context.Context, actorID string, credentials map[string]interface{},
	) *common.ServiceError
	GetActor(actorID string) (*Entity, *common.ServiceError)
	GetActorGroups(actorID string) ([]EntityGroup, *common.ServiceError)
}

// I18nProviderInterface defines the interface for the i18n provider.
type I18nProviderInterface interface {
	ResolveTranslations(
		ctx context.Context,
		language string,
		namespace string,
	) (*LanguageTranslationsResponse, *common.ServiceError)
	ListLanguages(ctx context.Context) ([]string, *common.ServiceError)
}

// DesignResolveProviderInterface defines the interface for the design resolve service.
type DesignResolveProviderInterface interface {
	ResolveDesign(
		ctx context.Context, resolveType DesignResolveType, id string,
	) (*DesignResponse, *common.ServiceError)
}

// OrganizationUnitProvider defines the interface for the organization unit provider.
type OrganizationUnitProvider interface {
	GetOrganizationUnit(ctx context.Context, id string) (OrganizationUnit, *common.ServiceError)
	GetOrganizationUnitList(
		ctx context.Context, limit, offset int, f *common.FilterGroup,
	) (*OrganizationUnitListResponse, *common.ServiceError)
	CreateOrganizationUnit(
		ctx context.Context, request OrganizationUnitRequestWithID,
	) (OrganizationUnit, *common.ServiceError)
	IsParent(ctx context.Context, parentID, childID string) (bool, *common.ServiceError)
	IsOrganizationUnitExists(ctx context.Context, id string) (bool, *common.ServiceError)
	GetOrganizationUnitChildren(
		ctx context.Context, id string, limit, offset int, f *common.FilterGroup,
	) (*OrganizationUnitListResponse, *common.ServiceError)
}

// FlowProviderInterface defines the flow management operations required for flow execution.
type FlowProviderInterface interface {
	GetFlowByHandle(ctx context.Context, handle string, flowType FlowType) (
		*CompleteFlowDefinition, *common.ServiceError)
	GetFlow(ctx context.Context, flowID string) (*CompleteFlowDefinition, *common.ServiceError)
}

// ResourceProviderInterface defines the interface for the resource provider.
type ResourceProviderInterface interface {
	GetResourceServerByIdentifier(
		ctx context.Context, identifier string,
	) (*ResourceServer, *common.ServiceError)
	ValidatePermissions(
		ctx context.Context, resourceServerID string, permissions []string,
	) ([]string, *common.ServiceError)
	FindResourceServersByPermissions(
		ctx context.Context, permissions []string,
	) ([]ResourceServer, *common.ServiceError)
}

// RoleProvider defines the interface for the role provider.
type RoleProvider interface {
	GetAuthorizedPermissions(
		ctx context.Context, entityID string, groups []string, requestedPermissions []string,
	) ([]string, *common.ServiceError)
}

// IDPProvider defines the interface for the identity provider provider.
type IDPProvider interface {
	GetIdentityProvidersByProperty(ctx context.Context, propertyKey,
		propertyValue string) ([]IDPDTO, *common.ServiceError)
	GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, *common.ServiceError)
}
