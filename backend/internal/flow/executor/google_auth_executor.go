// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	authngoogle "github.com/thunder-id/thunderid/internal/authn/google"
	authnoidc "github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// googleOIDCAuthExecutor implements the OIDC authentication executor for Google.
type googleOIDCAuthExecutor struct {
	oidcAuthExecutorInterface
	googleAuthService authngoogle.GoogleOIDCAuthnServiceInterface
}

var _ providers.Executor = (*googleOIDCAuthExecutor)(nil)

// newGoogleOIDCAuthExecutor creates a new instance of GoogleOIDCAuthExecutor with the provided details.
func newGoogleOIDCAuthExecutor(
	flowFactory core.FlowFactoryInterface,
	idpService providers.IDPProvider,
	authService authngoogle.GoogleOIDCAuthnServiceInterface,
	authnProvider providers.AuthnProviderManager,
) oidcAuthExecutorInterface {
	defaultInputs := []providers.Input{
		{
			Identifier: "code",
			Type:       "string",
			Required:   true,
		},
	}

	oidcSvcCast, ok := authService.(authnoidc.OIDCAuthnCoreServiceInterface)
	if !ok {
		panic("failed to cast GoogleOIDCAuthnService to OIDCAuthnCoreServiceInterface")
	}

	base := newOIDCAuthExecutor(ExecutorNameGoogleAuth, defaultInputs, []providers.Input{},
		flowFactory, idpService, oidcSvcCast, authnProvider, providers.IDPTypeGoogle)

	return &googleOIDCAuthExecutor{
		oidcAuthExecutorInterface: base,
		googleAuthService:         authService,
	}
}
