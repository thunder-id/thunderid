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

package thunderidengine

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/host"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/runtime"
)

// actorAdapter implements the internal actorprovider.ActorProviderInterface by delegating to a
// host.ActorProvider supplied by the embedder. It lives inside the engine module so it may
// construct the internal DTOs (entityprovider.Entity, inboundmodel.OAuthClient/InboundClient) that
// an external module cannot name. Write operations are not supported through the host SDK and
// return ErrorCodeNotImplemented.
type actorAdapter struct {
	h host.ActorProvider
}

// newActorAdapter wraps a host.ActorProvider as the internal actor provider interface.
func newActorAdapter(h host.ActorProvider) actorprovider.ActorProviderInterface {
	return &actorAdapter{h: h}
}

// --- entityprovider.EntityResolverInterface ---

func (a *actorAdapter) IdentifyEntity(filters map[string]interface{}) (*string, *entityprovider.EntityProviderError) {
	id, err := a.h.IdentifyEntity(filters)
	if err != nil {
		return nil, mapEntityErr(err, "failed to identify entity")
	}
	return id, nil
}

func (a *actorAdapter) SearchEntities(
	filters map[string]interface{},
) ([]*entityprovider.Entity, *entityprovider.EntityProviderError) {
	actors, err := a.h.SearchEntities(filters)
	if err != nil {
		return nil, mapEntityErr(err, "failed to search entities")
	}
	out := make([]*entityprovider.Entity, 0, len(actors))
	for _, ac := range actors {
		out = append(out, entityFromActor(ac))
	}
	return out, nil
}

func (a *actorAdapter) GetEntity(entityID string) (*entityprovider.Entity, *entityprovider.EntityProviderError) {
	ac, err := a.h.GetEntity(entityID)
	if err != nil {
		if errors.Is(err, runtime.ErrNotFound) {
			return nil, nil
		}
		return nil, mapEntityErr(err, "failed to get entity")
	}
	return entityFromActor(ac), nil
}

func (a *actorAdapter) CreateEntity(
	_ *entityprovider.Entity, _ json.RawMessage,
) (*entityprovider.Entity, *entityprovider.EntityProviderError) {
	return nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented,
		"create entity not supported", "the host ActorProvider is read-only")
}

func (a *actorAdapter) UpdateCredentials(_ string, _ json.RawMessage) *entityprovider.EntityProviderError {
	return entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented,
		"update credentials not supported", "the host ActorProvider is read-only")
}

func (a *actorAdapter) UpdateAttributes(_ string, _ json.RawMessage) *entityprovider.EntityProviderError {
	return entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented,
		"update attributes not supported", "the host ActorProvider is read-only")
}

func (a *actorAdapter) GetTransitiveEntityGroups(
	_ string,
) ([]entityprovider.EntityGroup, *entityprovider.EntityProviderError) {
	// The host SDK does not model groups; return an empty set.
	return []entityprovider.EntityGroup{}, nil
}

// --- actorprovider.ActorProviderInterface extras ---

func (a *actorAdapter) GetActor(actorID string) (*entityprovider.Entity, *entityprovider.EntityProviderError) {
	return a.GetEntity(actorID)
}

func (a *actorAdapter) GetActorGroups(
	_ string,
) ([]entityprovider.EntityGroup, *entityprovider.EntityProviderError) {
	return []entityprovider.EntityGroup{}, nil
}

func (a *actorAdapter) GetOAuthClientByClientID(
	ctx context.Context, clientID string,
) (*inboundmodel.OAuthClient, *serviceerror.ServiceError) {
	hc, err := a.h.GetInboundClientByClientID(ctx, clientID)
	if err != nil {
		if hc2, err2 := a.h.GetInboundClientByEntityID(ctx, clientID); err2 == nil {
			hc = hc2
		} else {
			// Absent or error: treat as unknown client (nil) so the OAuth layer rejects it.
			return nil, nil
		}
	}
	return oauthClientFromHost(hc), nil
}

func (a *actorAdapter) GetOAuthProfileByID(
	_ context.Context, _ string,
) (*inboundmodel.OAuthProfile, *serviceerror.ServiceError) {
	return nil, nil
}

func (a *actorAdapter) GetInboundClientByID(
	ctx context.Context, id string,
) (*inboundmodel.InboundClient, *serviceerror.ServiceError) {
	hc, err := a.h.GetInboundClientByEntityID(ctx, id)
	if err != nil {
		if hc2, err2 := a.h.GetInboundClientByClientID(ctx, id); err2 == nil {
			hc = hc2
		} else {
			return nil, nil
		}
	}
	return inboundClientFromHost(hc), nil
}

// --- mapping helpers ---

func entityFromActor(ac *host.Actor) *entityprovider.Entity {
	if ac == nil {
		return nil
	}
	return &entityprovider.Entity{
		ID:               ac.ID,
		Type:             ac.EntityType,
		OUID:             ac.OUID,
		Attributes:       ac.Attributes,
		SystemAttributes: ac.SystemAttributes,
	}
}

func oauthClientFromHost(hc *host.InboundClient) *inboundmodel.OAuthClient {
	if hc == nil {
		return nil
	}
	oc := &inboundmodel.OAuthClient{
		ID:                      hc.EntityID,
		OUID:                    hc.OUID,
		ClientID:                hc.ClientID,
		RedirectURIs:            append([]string(nil), hc.RedirectURIs...),
		GrantTypes:              toGrantTypes(hc.GrantTypes),
		ResponseTypes:           toResponseTypes(hc.ResponseTypes),
		TokenEndpointAuthMethod: oauth2const.TokenEndpointAuthMethod(hc.TokenEndpointAuthMethod),
		PKCERequired:            hc.PKCERequired,
		PublicClient:            hc.PublicClient,
	}
	if hc.Certificate != nil {
		oc.Certificate = &inboundmodel.Certificate{
			Type:  cert.CertificateType(hc.Certificate.Type),
			Value: hc.Certificate.Value,
		}
	}
	return oc
}

func inboundClientFromHost(hc *host.InboundClient) *inboundmodel.InboundClient {
	if hc == nil {
		return nil
	}

	properties := make(map[string]interface{})
	properties["logo_url"] = hc.LogoURL
	properties["url"] = hc.URL
	properties["tos_uri"] = hc.TosURI
	properties["policy_uri"] = hc.PolicyURI
	properties["name"] = hc.Name
	return &inboundmodel.InboundClient{
		ID:                        hc.EntityID,
		AuthFlowID:                hc.AuthFlowID,
		RegistrationFlowID:        hc.RegistrationFlowID,
		IsRegistrationFlowEnabled: hc.IsRegistrationFlowEnabled,
		RecoveryFlowID:            hc.RecoveryFlowID,
		IsRecoveryFlowEnabled:     hc.IsRecoveryFlowEnabled,
		Properties:                properties,
	}
}

func toGrantTypes(in []string) []oauth2const.GrantType {
	out := make([]oauth2const.GrantType, 0, len(in))
	for _, g := range in {
		out = append(out, oauth2const.GrantType(g))
	}
	return out
}

func toResponseTypes(in []string) []oauth2const.ResponseType {
	out := make([]oauth2const.ResponseType, 0, len(in))
	for _, r := range in {
		out = append(out, oauth2const.ResponseType(r))
	}
	return out
}

func mapEntityErr(err error, msg string) *entityprovider.EntityProviderError {
	if errors.Is(err, runtime.ErrNotFound) {
		return entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, msg, err.Error())
	}
	return entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, msg, err.Error())
}
