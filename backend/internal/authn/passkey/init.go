// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package passkey

import (
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the WebAuthn authentication service.
func Initialize(
	entitySvc entity.EntityServiceInterface,
	runtimeStore providers.RuntimeStoreProvider,
	governance identitygovernance.IdentityGovernanceServiceInterface,
) PasskeyServiceInterface {
	return newPasskeyService(entitySvc, newSessionStore(runtimeStore), governance)
}
