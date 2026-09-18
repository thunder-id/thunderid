// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package dataplane holds the surfaces that serve traffic.
//
// It is a package rather than a function in the shared build so that the separation is enforced by
// the linker: a binary that does not import this one does not contain authentication, OAuth2, flow
// execution or credential issuance. That is checkable, which a naming convention is not.
package dataplane

import (
	"context"

	"github.com/thunder-id/thunderid/internal/attestation"
	"github.com/thunder-id/thunderid/internal/authn"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authzen"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/oauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dcr"
	"github.com/thunder-id/thunderid/internal/openid4vci"
	"github.com/thunder-id/thunderid/internal/server"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// WireRuntime mounts the surfaces that serve traffic: authentication, access evaluation, flow
// execution, OAuth2 and the credential issuer.
//
// A plane that does not call this serves none of them, and links none of them either. That is the
// whole of the separation: it is not a setting the server reads, it is which function a binary calls.
func WireRuntime(svcs *server.Services) {
	ctx := context.Background()
	logger := svcs.Logger

	_, directAuthGuard := authn.Initialize(svcs.Mux, svcs.MCPServer, svcs.IDPService, svcs.JWTService,
		svcs.AuthnProvider, svcs.AuthAssertGen, svcs.OTPCoreService, svcs.NotifSenderSvc,
		svcs.TemplateService, svcs.MagicLinkService, svcs.OAuthAuthnService,
		svcs.OIDCAuthnService, svcs.GoogleAuthnService, svcs.GitHubAuthnService,
		svcs.DirectAuthSecret)

	// AuthZEN access-evaluation endpoints are Direct API endpoints, so they reuse the Direct Auth
	// guard created by the authn service.
	authzen.Initialize(svcs.Mux, svcs.AuthZService, svcs.EntityProvider, svcs.ResourceService,
		directAuthGuard)

	// The OpenID4VP verifier exists already; this is where its wallet and verifier endpoints start
	// answering. A control plane holds the same service and mounts none of them.
	openid4vp.RegisterRoutes(svcs.Mux, svcs.OpenID4VPSvc)

	attestationProvider := initAttestationProvider(ctx, logger, svcs.RuntimeCryptoSvc)
	flowExecService, err := flowexec.Initialize(svcs.Mux, svcs.FlowMgtService, svcs.ActorProvider,
		svcs.ExecRegistry, svcs.InterceptorRegistry, svcs.ObservabilitySvc, svcs.RuntimeCryptoSvc,
		attestationProvider, svcs.GraphBuilder, svcs.JWTService, svcs.RuntimeStoreProvider,
		svcs.Transactioner, svcs.ServerConfigService, svcs.FlowConfig)
	server.FatalOnError(ctx, logger, err, "Failed to initialize flow execution service")

	// Initialize OAuth services.
	tokenValidator, err := oauth.Initialize(svcs.Mux, svcs.ActorProvider, svcs.AuthnProvider,
		svcs.JWTService, svcs.JWEService, flowExecService, svcs.ObservabilitySvc, svcs.RuntimeCryptoSvc,
		svcs.OUService, svcs.AttributeCacheService, svcs.AuthZService, svcs.ResourceServerProvider,
		svcs.I18nService, svcs.IDPService, svcs.DPoPVerifier, svcs.RuntimeStoreProvider,
		svcs.Transactioner, svcs.RevocationEnforcer, svcs.RevocationSvc, svcs.SessionService,
		svcs.FlowMgtService, svcs.OAuthCfg)
	server.FatalOnError(ctx, logger, err, "Failed to initialize OAuth services")

	// Initialized after the OAuth services because credential issuance validates the presented
	// access token with the OAuth token validator and resolves the wallet application behind it.
	_, err = openid4vci.Initialize(svcs.Mux, svcs.RuntimeCryptoSvc, tokenValidator, svcs.UserService,
		svcs.DPoPVerifier, svcs.OpenID4VCICredSvc, svcs.ActorProvider, svcs.RuntimeStoreProvider)
	server.FatalOnError(ctx, logger, err, "Failed to initialize OpenID4VCI issuer service")

	if svcs.OAuthCfg.OAuth.DCR.IsEnabled() {
		// Register OAuth2 DCR service.
		err = dcr.Initialize(svcs.Mux, svcs.ApplicationService, svcs.OUService, svcs.I18nService,
			svcs.OAuthCfg)
		server.FatalOnError(ctx, logger, err, "Failed to initialize OAuth2 DCR service")
	}
}

// initAttestationProvider initializes the platform attestation provider, terminating server startup
// on failure rather than running with a non-functional verifier.
func initAttestationProvider(ctx context.Context, logger *log.Logger,
	cryptoSvc providers.RuntimeCryptoProvider) providers.AttestationProvider {
	attestationProvider, err := attestation.Initialize(cryptoSvc)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize attestation provider", log.Error(err))
	}
	return attestationProvider
}
