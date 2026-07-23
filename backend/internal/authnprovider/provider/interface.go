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

// Package provider defines the authentication provider interface implemented by
// each concrete authn provider.
package provider

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AuthnProviderInterface defines the interface for authentication providers.
type AuthnProviderInterface interface {
	InitiateAuthentication(ctx context.Context, credentialType string, initData any,
		metadata *providers.AuthnMetadata) (any, *tidcommon.ServiceError)
	Authenticate(ctx context.Context, identifiers, credentials map[string]interface{},
		metadata *providers.AuthnMetadata) (*providers.AuthnResult, *tidcommon.ServiceError)
	GetEntityReference(ctx context.Context, entityReferenceToken any) (*providers.EntityReference,
		*tidcommon.ServiceError)
	GetAttributes(ctx context.Context, attributeToken any, consentedAttributes *providers.RequestedAttributes,
		metadata *providers.GetAttributesMetadata) (
		*providers.AttributesResponse, *tidcommon.ServiceError)
	InitiateEnrollment(ctx context.Context, credentialType string, initData any,
		metadata *providers.AuthnMetadata) (any, *tidcommon.ServiceError)
	Enroll(ctx context.Context, identifiers, credentials map[string]interface{},
		metadata *providers.AuthnMetadata) (*providers.AuthnResult, *tidcommon.ServiceError)
}
