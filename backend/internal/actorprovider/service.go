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

package actorprovider

import (
	"context"
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// actorProvider delegates actor resolution to inbound-client and entity-provider services, and
// actor authentication to the authentication provider.
type actorProvider struct {
	inboundClient  inboundclient.InboundClientServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  providers.AuthnProviderManager
	logger         *log.Logger
}

// newActorProvider creates a new actorProvider backed by the given inbound-client, entity-provider,
// and authentication provider.
func newActorProvider(
	inboundClient inboundclient.InboundClientServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
) providers.ActorProvider {
	return &actorProvider{
		inboundClient:  inboundClient,
		entityProvider: entityProvider,
		authnProvider:  authnProvider,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ActorProvider")),
	}
}

// GetOAuthClientByClientID returns the OAuth client registered for the given ID.
func (p *actorProvider) GetOAuthClientByClientID(
	ctx context.Context, clientID string,
) (*providers.OAuthClient, *tidcommon.ServiceError) {
	client, err := p.inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		if errors.Is(err, inboundclient.ErrInboundClientNotFound) {
			return nil, &ErrorActorNotFound
		}
		p.logger.Error(ctx, "Failed to fetch OAuth client", log.String("clientID", clientID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return toProviderOAuthClient(client), nil
}

// GetOAuthProfileByID returns the stored OAuth profile for the given entity UUID.
func (p *actorProvider) GetOAuthProfileByID(
	ctx context.Context, id string,
) (*providers.OAuthProfile, *tidcommon.ServiceError) {
	profile, err := p.inboundClient.GetOAuthProfileByEntityID(ctx, id)
	if err != nil {
		if errors.Is(err, inboundclient.ErrInboundClientNotFound) {
			return nil, &ErrorActorNotFound
		}
		p.logger.Error(ctx, "Failed to fetch OAuth profile by entity ID",
			log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return profile, nil
}

// GetInboundClientByID returns the inbound-client row for the given ID.
func (p *actorProvider) GetInboundClientByID(
	ctx context.Context, id string,
) (*providers.InboundClient, *tidcommon.ServiceError) {
	client, err := p.inboundClient.GetInboundClientByEntityID(ctx, id)
	if err != nil {
		if errors.Is(err, inboundclient.ErrInboundClientNotFound) {
			return nil, &ErrorActorNotFound
		}
		p.logger.Error(ctx, "Failed to fetch inbound client", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return client, nil
}

// AuthenticateActor verifies the supplied credentials against the actor resolved from the given
// identifiers. It returns nil when authentication succeeds and a service error otherwise. The
// default implementation delegates to the authentication provider, which performs the credential
// lookup and constant-time verification generically through the entity layer. A custom actor
// provider may implement its own scheme.
func (p *actorProvider) AuthenticateActor(
	ctx context.Context, identifiers, credentials map[string]interface{},
) *tidcommon.ServiceError {
	_, _, svcErr := p.authnProvider.AuthenticateUser(ctx, identifiers, credentials, nil, nil, providers.AuthUser{})
	return svcErr
}

// GetActor returns the backing entity record for the given actor ID.
func (p *actorProvider) GetActor(actorID string) (*providers.Entity, *tidcommon.ServiceError) {
	entity, epErr := p.entityProvider.GetEntity(actorID)
	if epErr != nil {
		return nil, mapEntityProviderError(epErr)
	}
	return entity, nil
}

// GetActorGroups returns transitive group memberships for the given actor ID.
func (p *actorProvider) GetActorGroups(
	actorID string,
) ([]providers.EntityGroup, *tidcommon.ServiceError) {
	groups, epErr := p.entityProvider.GetTransitiveEntityGroups(actorID)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			return nil, nil
		}
		return nil, mapEntityProviderError(epErr)
	}
	return groups, nil
}

func mapEntityProviderError(epErr *entityprovider.EntityProviderError) *tidcommon.ServiceError {
	if epErr == nil {
		return nil
	}
	switch epErr.Code {
	case entityprovider.ErrorCodeEntityNotFound:
		return &ErrorEntityNotFound
	default:
		return &tidcommon.InternalServerError
	}
}

func toProviderOAuthClient(c *providers.OAuthClient) *providers.OAuthClient {
	if c == nil {
		return nil
	}
	client := &providers.OAuthClient{
		ID:                                 c.ID,
		OUID:                               c.OUID,
		ClientID:                           c.ClientID,
		RedirectURIs:                       c.RedirectURIs,
		TokenEndpointAuthMethod:            c.TokenEndpointAuthMethod,
		PKCERequired:                       c.PKCERequired,
		PublicClient:                       c.PublicClient,
		RequirePushedAuthorizationRequests: c.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              c.DPoPBoundAccessTokens,
		IncludeActClaim:                    c.IncludeActClaim,
		EntityCategory:                     c.EntityCategory,
		Token:                              c.Token,
		Scopes:                             c.Scopes,
		UserInfo:                           c.UserInfo,
		ScopeClaims:                        c.ScopeClaims,
		Certificate:                        c.Certificate,
		AcrValues:                          c.AcrValues,
	}
	client.GrantTypes = append(client.GrantTypes, c.GrantTypes...)
	client.ResponseTypes = append(client.ResponseTypes, c.ResponseTypes...)
	return client
}
