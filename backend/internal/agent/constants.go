// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	agentBasePath = "/agents"
)

// Field keys stored in the entity SystemAttributes JSON blob.
const (
	fieldDescription  = "description"
	fieldOwner        = "owner"
	fieldClientID     = "clientId"
	fieldClientSecret = authnprovidercm.CredentialTypeClientSecret
)

// propLogoURL is the inbound-client PROPERTIES key holding the agent logo.
const propLogoURL = "logo_url"

// defaultGrantTypes are the grant types every agent holds.
var defaultGrantTypes = []providers.GrantType{
	providers.GrantTypeClientCredentials,
}

// delegationGrantTypes are the grant types that let an agent act for a user. A delegated agent needs
// at least one of them.
var delegationGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeCIBA,
	providers.GrantTypeTokenExchange,
	providers.GrantTypeJWTBearer,
}

// defaultDelegatedGrantTypes are the grant types added to a delegated agent that requests none
// beyond client credentials.
var defaultDelegatedGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeRefreshToken,
}

// lifespanFailureClass is the failure class AGENT_LIFESPAN_UPDATE_FAILED reports.
type lifespanFailureClass string

// Failure classes reported by AGENT_LIFESPAN_UPDATE_FAILED.
const (
	lifespanFailureNotAuthorized   lifespanFailureClass = "not_authorized"
	lifespanFailureInvalidLifespan lifespanFailureClass = "invalid_lifespan"
	lifespanFailureReadOnly        lifespanFailureClass = "read_only"
	lifespanFailureOperationFailed lifespanFailureClass = "operation_failed"
)
