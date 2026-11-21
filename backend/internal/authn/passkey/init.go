// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package passkey

import (
	authnconfig "github.com/thunder-id/thunderid/internal/authn/config"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the WebAuthn authentication service.
func Initialize(
	entitySvc entity.EntityServiceInterface,
	runtimeStore providers.RuntimeStoreProvider,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	cfg authnconfig.Config,
) PasskeyServiceInterface {
	return newPasskeyService(entitySvc, newSessionStore(runtimeStore), governance, cfg.PasskeyAllowedOrigins)
}
