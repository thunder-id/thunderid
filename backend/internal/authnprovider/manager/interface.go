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

// Package manager provides a manager layer between callers and concrete authentication provider implementations.
package manager

import (
	"context"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// AuthnProviderManagerInterface defines the interface for the authentication provider manager.
type AuthnProviderManagerInterface interface {
	AuthenticateUser(ctx context.Context, identifiers, credentials map[string]interface{},
		requestedAttributes *authnprovidercm.RequestedAttributes,
		metadata *authnprovidercm.AuthnMetadata,
		authUser AuthUser) (AuthUser, *AuthnBasicResult, *serviceerror.ServiceError)
	GetUserAvailableAttributes(ctx context.Context,
		authUser AuthUser) (*authnprovidercm.AttributesResponse, *serviceerror.ServiceError)
	GetUserAttributes(ctx context.Context,
		requestedAttributes *authnprovidercm.RequestedAttributes,
		metadata *authnprovidercm.GetAttributesMetadata,
		authUser AuthUser) (AuthUser, *authnprovidercm.AttributesResponse, *serviceerror.ServiceError)
}
