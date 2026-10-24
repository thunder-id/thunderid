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
	// fieldKind holds the agent kind. It is separate from the entity category, which is
	// "agent" for every agent. It is set only when the agent is created and carried forward on every
	// update, so writes to an agent's system attributes must go through the agent service.
	fieldKind = "kind"
)

// Defaults for the server-config "personalAgent" section.
const (
	// defaultPersonalAgentMaxAccessTokenValidity caps, in seconds, a personal agent's access tokens when
	// the organization sets no cap.
	defaultPersonalAgentMaxAccessTokenValidity int64 = 3600
	// defaultPersonalAgentAuthFlowHandle is the shipped login flow for a personal agent whose
	// registration flow fixes none.
	defaultPersonalAgentAuthFlowHandle = "default-personal-agent-auth-flow"
)

// propLogoURL is the inbound-client PROPERTIES key holding the agent logo.
const propLogoURL = "logo_url"

// defaultGrantTypes are the grant types every agent holds.
var defaultGrantTypes = []providers.GrantType{
	providers.GrantTypeClientCredentials,
}

// delegationGrantTypes are the grant types that let an agent act for a signed-in user. A delegated
// agent needs at least one of them.
var delegationGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeCIBA,
	providers.GrantTypeTokenExchange,
}

// defaultDelegatedGrantTypes are the grant types added to a delegated agent that requests none
// beyond client credentials.
var defaultDelegatedGrantTypes = []providers.GrantType{
	providers.GrantTypeAuthorizationCode,
	providers.GrantTypeRefreshToken,
}
