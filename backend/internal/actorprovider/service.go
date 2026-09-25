// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package actorprovider

import (
	"context"
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/role"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// actorProvider delegates actor resolution to inbound-client and entity-provider services, and
// actor authentication to the authentication provider.
type actorProvider struct {
	inboundClient  inboundclient.InboundClientServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	authnProvider  providers.AuthnProviderManager
	roleService    role.RoleServiceInterface
	// appService answers which organization units an application may be used on behalf of. It is nil
	// outside the server, which hosts no application management; that refuses every organization unit
	// but the application's own.
	appService application.ApplicationServiceInterface
	logger     *log.Logger
}

// newActorProvider creates a new actorProvider backed by the given inbound-client, entity-provider,
// authentication provider, role and application services.
func newActorProvider(
	inboundClient inboundclient.InboundClientServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	authnProvider providers.AuthnProviderManager,
	roleService role.RoleServiceInterface,
	appService application.ApplicationServiceInterface,
) providers.ActorProvider {
	return &actorProvider{
		inboundClient:  inboundClient,
		entityProvider: entityProvider,
		authnProvider:  authnProvider,
		roleService:    roleService,
		appService:     appService,
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

	oauthClient := toProviderOAuthClient(client)
	if svcErr := p.requireUsableInAccessingOU(ctx, oauthClient); svcErr != nil {
		return nil, svcErr
	}
	return oauthClient, nil
}

// requireUsableInAccessingOU refuses a client the request may not act as.
//
// Resolution and admission are one step on purpose. The lookup itself stays global, so one
// registration still serves every organization unit and no per-customer registration is needed;
// what is scoped is whether the client may be used. Doing it here rather than in a grant handler
// means every path that resolves a client inherits the rule instead of each handler remembering to
// ask.
func (p *actorProvider) requireUsableInAccessingOU(
	ctx context.Context, oauthClient *providers.OAuthClient,
) *tidcommon.ServiceError {
	accessingOUID := syscontext.GetAccessingOUID(ctx)
	if accessingOUID == "" || oauthClient == nil {
		return nil
	}

	// An application's own organization unit needs no policy of its own. The framework would answer
	// the same, but only after resolving ownership, and this is the common case on the token path.
	if oauthClient.OUID == accessingOUID {
		return nil
	}

	if p.appService == nil {
		return &tidcommon.ErrorUnauthorized
	}

	visible, svcErr := p.appService.IsApplicationVisibleToOU(ctx, oauthClient.ID, accessingOUID)
	if svcErr != nil {
		p.logger.Error(ctx, "Failed to resolve application access for an organization unit",
			log.String("appID", oauthClient.ID), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !visible {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
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

// GetTransitiveGroupAncestors returns the ancestor chain of a single group.
func (p *actorProvider) GetTransitiveGroupAncestors(
	groupID string,
) ([]string, *tidcommon.ServiceError) {
	ancestors, epErr := p.entityProvider.GetTransitiveGroupAncestors(groupID)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeNotImplemented {
			return nil, nil
		}
		return nil, mapEntityProviderError(epErr)
	}
	return ancestors, nil
}

// GetActorRoles returns the roles assigned to the actor, directly and through the given groups.
func (p *actorProvider) GetActorRoles(
	actorID string, groupIDs []string,
) ([]string, *tidcommon.ServiceError) {
	if p.roleService == nil {
		return nil, nil
	}
	return p.roleService.GetUserRoles(context.Background(), actorID, groupIDs)
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
		PostLogoutRedirectURIs:             c.PostLogoutRedirectURIs,
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
