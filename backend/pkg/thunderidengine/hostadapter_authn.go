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

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/host"
)

// authnAdapter implements the internal authnprovidermgr.AuthnProviderManagerInterface by
// delegating to a host.AuthnProvider supplied by the embedder. It lives inside the engine module
// so it can construct the otherwise-opaque authnprovidermgr.AuthUser (whose fields are unexported)
// via its JSON proxy, and reference the internal authnprovidercm DTOs an external module cannot
// name.
type authnAdapter struct {
	h host.AuthnProvider
}

// newAuthnAdapter wraps a host.AuthnProvider as the internal authn provider manager interface.
func newAuthnAdapter(h host.AuthnProvider) authnprovidermgr.AuthnProviderManagerInterface {
	return &authnAdapter{h: h}
}

func (a *authnAdapter) AuthenticateUser(
	ctx context.Context,
	identifiers, credentials map[string]interface{},
	_ *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.AuthnMetadata,
	_ authnprovidermgr.AuthUser,
) (authnprovidermgr.AuthUser, authnprovidercm.AuthenticatedClaims, *serviceerror.ServiceError) {
	res, err := a.h.Authenticate(ctx, identifiers, credentials, mapAuthnMetadata(metadata))
	if err != nil || res == nil || !res.Authenticated {
		e := authnprovidermgr.ErrorAuthenticationFailed
		return authnprovidermgr.AuthUser{}, nil, &e
	}

	entityRef := &authnprovidercm.EntityReference{
		EntityID:       res.UserID,
		EntityCategory: "user",
	}
	var attrs *authnprovidercm.AttributesResponse
	if len(res.Attributes) > 0 {
		attrs = attrsFromRaw(res.Attributes)
	}
	// The host auth token is stashed as the attribute token so GetUserAttributes can fetch
	// attributes later; it also satisfies the attribute side of AuthUser.IsAuthenticated even
	// when no attributes are returned inline.
	au := authnprovidermgr.NewAuthUser(entityRef, nil, attrs, res.AuthToken)
	return au, claimsFromRaw(res.Attributes), nil
}

func (a *authnAdapter) GetEntityReference(
	_ context.Context, authUser authnprovidermgr.AuthUser,
) (authnprovidermgr.AuthUser, *authnprovidercm.EntityReference, *serviceerror.ServiceError) {
	ref := authUser.EntityReference()
	if ref == nil {
		e := authnprovidermgr.ErrorGetEntityReferenceClientError
		return authUser, nil, &e
	}
	return authUser, ref, nil
}

func (a *authnAdapter) GetUserAvailableAttributes(
	ctx context.Context, authUser authnprovidermgr.AuthUser,
) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if attrs := authUser.Attributes(); attrs != nil {
		return attrs, nil
	}
	token, _ := authUser.AttributeToken().(string)
	res, err := a.h.GetAttributes(ctx, token, nil, nil)
	if err != nil || res == nil {
		e := authnprovidermgr.ErrorGetAttributesClientError
		return nil, &e
	}
	return attrsFromRaw(res.Attributes), nil
}

func (a *authnAdapter) GetUserAttributes(
	ctx context.Context,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	metadata *authnprovidercm.GetAttributesMetadata,
	authUser authnprovidermgr.AuthUser,
) (authnprovidermgr.AuthUser, *authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	token, _ := authUser.AttributeToken().(string)
	res, err := a.h.GetAttributes(ctx, token, mapReqAttrs(requestedAttributes), mapGetAttrsMetadata(metadata))
	if err != nil || res == nil {
		e := authnprovidermgr.ErrorGetAttributesClientError
		return authUser, nil, &e
	}
	attrs := attrsFromRaw(res.Attributes)
	authUser = authnprovidermgr.NewAuthUser(
		authUser.EntityReference(), authUser.EntityReferenceToken(), attrs, authUser.AttributeToken())
	return authUser, attrs, nil
}

// --- mapping helpers ---

func mapAuthnMetadata(m *authnprovidercm.AuthnMetadata) *host.AuthnMetadata {
	if m == nil {
		return nil
	}
	return &host.AuthnMetadata{AppMetadata: m.AppMetadata, RuntimeMetadata: m.RuntimeMetadata}
}

func mapGetAttrsMetadata(m *authnprovidercm.GetAttributesMetadata) *host.GetAttributesMetadata {
	if m == nil {
		return nil
	}
	return &host.GetAttributesMetadata{AppMetadata: m.AppMetadata, Locale: m.Locale, RuntimeMetadata: m.RuntimeMetadata}
}

func mapReqAttrs(r *authnprovidercm.RequestedAttributes) *host.RequestedAttributes {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.Attributes))
	for name := range r.Attributes {
		names = append(names, name)
	}
	return &host.RequestedAttributes{Names: names}
}

// attrsFromRaw converts a flat JSON object {attr: value} into an AttributesResponse.
func attrsFromRaw(raw json.RawMessage) *authnprovidercm.AttributesResponse {
	if len(raw) == 0 {
		return &authnprovidercm.AttributesResponse{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return &authnprovidercm.AttributesResponse{}
	}
	attrs := make(map[string]*authnprovidercm.AttributeResponse, len(m))
	for k, v := range m {
		attrs[k] = &authnprovidercm.AttributeResponse{Value: v}
	}
	return &authnprovidercm.AttributesResponse{Attributes: attrs}
}

// claimsFromRaw converts a flat JSON object into AuthenticatedClaims.
func claimsFromRaw(raw json.RawMessage) authnprovidercm.AuthenticatedClaims {
	claims := authnprovidercm.AuthenticatedClaims{}
	if len(raw) == 0 {
		return claims
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err == nil {
		for k, v := range m {
			claims[k] = v
		}
	}
	return claims
}
