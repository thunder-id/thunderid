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

// Package provider provides authentication provider implementations.
package provider

import (
	"context"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// AuthnProviderInterface defines the interface for authentication providers.
type AuthnProviderInterface interface {
	Authenticate(ctx context.Context, identifiers, credentials map[string]interface{},
		metadata *authnprovidercm.AuthnMetadata) (*authnprovidercm.AuthnResult, *serviceerror.ServiceError)
	GetAttributes(ctx context.Context, token string, requestedAttributes *authnprovidercm.RequestedAttributes,
		metadata *authnprovidercm.GetAttributesMetadata) (
		*authnprovidercm.GetAttributesResult, *serviceerror.ServiceError)
}
