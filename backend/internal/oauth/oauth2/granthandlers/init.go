/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package granthandlers

import (
	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// Initialize initializes the grant handler provider. oauth2AuthzService is created by the
// caller (oauth/init.go) so it can also be passed to the callback dispatcher without
// granthandlers needing to own authz initialization or expose it.
func Initialize(
	jwtService jwt.JWTServiceInterface,
	oauth2AuthzService oauth2authz.AuthorizeServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	authzService authz.AuthorizationServiceInterface,
	entityProv entityprovider.EntityProviderInterface,
	resourceService resource.ResourceServiceInterface,
	cibaService ciba.CIBAServiceInterface,
) GrantHandlerProviderInterface {
	return newGrantHandlerProvider(
		jwtService,
		oauth2AuthzService,
		tokenBuilder,
		tokenValidator,
		attrCacheService,
		ouService,
		authzService,
		entityProv,
		resourceService,
		cibaService,
	)
}
