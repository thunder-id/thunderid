// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package userinfo

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the userinfo handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	tokenValidator tokenservice.TokenValidatorInterface,
	actorProvider providers.ActorProvider,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	dpopVerifier dpop.VerifierInterface,
	cfg oauthconfig.Config,
) userInfoServiceInterface {
	userInfoService := newUserInfoService(jwtService, jweService, resolver, tokenValidator,
		actorProvider, attributeCacheSvc, dpopVerifier, cfg)
	userInfoEndpoint := cfg.BaseURL + constants.OAuth2UserInfoEndpoint
	dpopAlgs := cfg.OAuth.DPoP.AllowedAlgs
	userInfoHandler := newUserInfoHandler(userInfoService, userInfoEndpoint, dpopAlgs)
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

	userInfo := middleware.CorrelationIDMiddleware(http.HandlerFunc(userInfoHandler.HandleUserInfo)).ServeHTTP
	mux.HandleFunc(middleware.WithCORS("GET "+constants.OAuth2UserInfoEndpoint, userInfo, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+constants.OAuth2UserInfoEndpoint, userInfo, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2UserInfoEndpoint,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
