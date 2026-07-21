/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package tokenservice

import (
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the token service components (builder and validator).
// Returns both TokenBuilderInterface and TokenValidatorInterface for centralized token operations.
func Initialize(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	idpService providers.IDPProvider,
	enforcementService revocation.EnforcementServiceInterface,
) (TokenBuilderInterface, TokenValidatorInterface) {
	tokenBuilder := newTokenBuilder(cfg, jwtService, jweService, resolver)
	tokenValidator := newTokenValidator(cfg, jwtService, idpService, enforcementService)
	return tokenBuilder, tokenValidator
}
