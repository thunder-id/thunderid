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

package userinfo

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the userinfo handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	tokenValidator tokenservice.TokenValidatorInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	transactioner transaction.Transactioner,
) userInfoServiceInterface {
	userInfoService := newUserInfoService(jwtService, jweService, resolver, tokenValidator,
		inboundClient, ouService, attributeCacheSvc, transactioner)
	userInfoHandler := newUserInfoHandler(userInfoService)
	registerRoutes(mux, userInfoHandler)
	return userInfoService
}

// registerRoutes registers the routes for the UserInfo endpoint.
func registerRoutes(mux *http.ServeMux, userInfoHandler *userInfoHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET "+constants.OAuth2UserInfoEndpoint,
		userInfoHandler.HandleUserInfo, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+constants.OAuth2UserInfoEndpoint,
		userInfoHandler.HandleUserInfo, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2UserInfoEndpoint,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
