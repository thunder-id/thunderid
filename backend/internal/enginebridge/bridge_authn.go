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

package enginebridge

import (
	"context"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type authnBridge struct {
	provider thunderidengine.AuthnProvider
}

func newAuthnBridge(provider thunderidengine.AuthnProvider) *authnBridge {
	return &authnBridge{provider: provider}
}

func (b *authnBridge) AuthenticateUser(
	ctx context.Context, identifiers, credentials map[string]interface{},
	requestedAttributes *authnprovidercm.RequestedAttributes,
	_ *authnprovidercm.AuthnMetadata,
	authUser authnprovidermgr.AuthUser,
) (authnprovidermgr.AuthUser, *authnprovidermgr.AuthnBasicResult, *serviceerror.ServiceError) {
	if b.provider == nil {
		return authnprovidermgr.AuthUser{}, nil, &serviceerror.InternalServerError
	}
	_ = requestedAttributes
	creds := mergeCredentialMaps(identifiers, credentials)
	result, err := b.provider.AuthenticateUser(ctx, creds)
	if err != nil {
		return authnprovidermgr.AuthUser{}, nil, providerError(err)
	}
	if result == nil {
		return authnprovidermgr.AuthUser{}, nil, &serviceerror.InternalServerError
	}
	userID := result.UserID
	if userID == "" {
		userID = result.EntityID
	}
	if userID == "" {
		return authUser, &authnprovidermgr.AuthnBasicResult{
			ExternalSub:    result.EntityID,
			IsExistingUser: false,
		}, nil
	}
	userType := result.EntityType
	if userType == "" {
		userType = result.EntityCategory
	}
	authUser.ApplyIdentity(userID, userType, result.OUID)
	authUser.ApplyProviderSession(result.Token, attributesToResponse(result.Attributes), len(result.Attributes) > 0)
	return authUser, &authnprovidermgr.AuthnBasicResult{
		UserID:         userID,
		OUID:           result.OUID,
		UserType:       userType,
		IsExistingUser: true,
	}, nil
}

func (b *authnBridge) GetUserAvailableAttributes(
	ctx context.Context, authUser authnprovidermgr.AuthUser,
) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		return nil, &serviceerror.InternalServerError
	}
	if attrs, ok := authUser.CachedAttributes(); ok {
		return attrs, nil
	}
	return b.fetchAttributes(ctx, authUser, nil)
}

func (b *authnBridge) GetUserAttributes(
	ctx context.Context,
	requestedAttributes *authnprovidercm.RequestedAttributes,
	_ *authnprovidercm.GetAttributesMetadata,
	authUser authnprovidermgr.AuthUser,
) (authnprovidermgr.AuthUser, *authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if !authUser.IsAuthenticated() {
		return authnprovidermgr.AuthUser{}, nil, &serviceerror.InternalServerError
	}
	if attrs, ok := authUser.CachedAttributes(); ok && attrs != nil {
		return authUser, attrs, nil
	}
	attrs, svcErr := b.fetchAttributes(ctx, authUser, requestedAttributes)
	if svcErr != nil {
		return authnprovidermgr.AuthUser{}, nil, svcErr
	}
	return authUser, attrs, nil
}

func (b *authnBridge) fetchAttributes(
	ctx context.Context,
	authUser authnprovidermgr.AuthUser,
	requested *authnprovidercm.RequestedAttributes,
) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	names := attributeNames(requested)
	values, err := b.provider.GetUserAttributes(ctx, authUser.UserID(), names)
	if err != nil {
		return nil, providerError(err)
	}
	return attributesToResponse(values), nil
}

func mergeCredentialMaps(identifiers, credentials map[string]interface{}) thunderidengine.Credentials {
	out := make(thunderidengine.Credentials, len(identifiers)+len(credentials))
	for k, v := range identifiers {
		out[k] = v
	}
	for k, v := range credentials {
		out[k] = v
	}
	return out
}

func attributeNames(requested *authnprovidercm.RequestedAttributes) []string {
	if requested == nil || len(requested.Attributes) == 0 {
		return nil
	}
	names := make([]string, 0, len(requested.Attributes))
	for name := range requested.Attributes {
		names = append(names, name)
	}
	return names
}

var _ authnprovidermgr.AuthnProviderManagerInterface = (*authnBridge)(nil)
