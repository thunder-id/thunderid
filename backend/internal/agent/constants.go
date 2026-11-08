// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"regexp"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	agentBasePath = "/agents"
)

// projectIDQueryParam is the query parameter that lists the agents of one project.
const projectIDQueryParam = "projectId"

// Field keys stored in the entity SystemAttributes JSON blob.
const (
	fieldDescription  = "description"
	fieldOwner        = "owner"
	fieldClientID     = "clientId"
	fieldClientSecret = authnprovidercm.CredentialTypeClientSecret
)

// fieldOwnerID is the path of the owner's ID within the SystemAttributes blob.
const fieldOwnerID = fieldOwner + ".id"

// maxOwnerIDLength is the longest ID an agent owner may have. It keeps the owner well inside the
// owner index's row size on PostgreSQL.
const maxOwnerIDLength = 256

// ownerMobilePattern matches a mobile number in E.164 form.
var ownerMobilePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// ownerIDTypes are the ID types an agent owner may have.
var ownerIDTypes = []providers.AgentOwnerIDType{
	providers.AgentOwnerIDTypeUUID,
	providers.AgentOwnerIDTypeEmail,
	providers.AgentOwnerIDTypeMobile,
}

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

// agentCredential is how an agent proves who it is, in the terms the agent MCP tools use.
type agentCredential string

// Credentials the agent MCP tools offer.
const (
	credentialClientSecret agentCredential = "client_secret"
	//nolint:gosec // The name of a client authentication method, not a credential.
	credentialPrivateKeyJWT agentCredential = "private_key_jwt"
	credentialNone          agentCredential = "none"
)

// actingMode is a way an agent acts for users, in the terms the agent MCP tools use.
type actingMode string

// Ways of acting for users the agent MCP tools offer, in the order they are reported.
const (
	actingSignIn        actingMode = "sign_in"
	actingApproval      actingMode = "approval"
	actingTokenExchange actingMode = "token_exchange"
)

// actingModes lists every way of acting for users, in the order they are reported.
var actingModes = []actingMode{actingSignIn, actingApproval, actingTokenExchange}

// actingModeGrantTypes maps each way of acting for users to the grant type that provides it.
var actingModeGrantTypes = map[actingMode]providers.GrantType{
	actingSignIn:        providers.GrantTypeAuthorizationCode,
	actingApproval:      providers.GrantTypeCIBA,
	actingTokenExchange: providers.GrantTypeTokenExchange,
}

// maxKeySetLength is the longest key set value the certificate store accepts.
const maxKeySetLength = 4096

// agentDetail is a section get_agent adds to an agent only when asked.
type agentDetail string

// Sections get_agent adds when asked.
const (
	agentDetailGroups      agentDetail = "groups"
	agentDetailRoles       agentDetail = "roles"
	agentDetailPermissions agentDetail = "permissions"
)

// agentDetails lists every section get_agent can add, in the order it reports them.
var agentDetails = []agentDetail{agentDetailGroups, agentDetailRoles, agentDetailPermissions}

// Agent MCP tool defaults.
const (
	// defaultAgentNameAttribute holds an agent's name when its type names no display attribute.
	defaultAgentNameAttribute = "name"
	// defaultOUHandle is the handle of the organization unit an agent is created in when none is named.
	defaultOUHandle = "default"
	// defaultAgentPageSize is the number of agents a list returns when no limit is given.
	defaultAgentPageSize = 30
)

// accessTerms describes how an agent proves who it is and acts for users, in the terms onboard_agent
// and update_agent take.
const accessTerms = `- credential: how it proves who it is. client_secret issues a secret, returned once in ` +
	`the result and never again. private_key_jwt signs with the agent's own key, from jwksUri or jwks, so no ` +
	`secret exists; prefer it when the agent can hold a private key. none: no credential.
- actsForUsers: leave it empty if the agent acts only on its own. sign_in: a user who is present signs in ` +
	`and the agent acts in that session; needs redirectUris. approval: the agent asks a user who isn't ` +
	`there, or is offline, to approve on their own device (CIBA); needs authFlowId, a flow that can reach ` +
	`that user, and the error names the flows that can if the one given can't. token_exchange: the ` +
	`agent exchanges a user token it already holds, or takes sub-tasks from another agent. Any ` +
	`combination works, and approval and token_exchange need a credential. sign_in and approval share ` +
	`one authentication flow.`

// endpointsNote says where the paths in a token guide are served. Only a control plane registers
// onboard_agent, and it serves no token endpoints.
const endpointsNote = "This is a control plane, which serves no token endpoints. The paths below are on each " +
	"data plane the agent is deployed to, and its discovery document at /.well-known/openid-configuration " +
	"lists the issuer and every endpoint."

// secretNotice goes with every client secret a tool returns.
const secretNotice = "This is the only time this client secret is shown, and no tool can return it again. " +
	"Hand it to the administrator to keep in a secret manager, and don't repeat it in the conversation."

// lifespanFailureClass is the failure class AGENT_LIFESPAN_UPDATE_FAILED reports.
type lifespanFailureClass string

// Failure classes reported by AGENT_LIFESPAN_UPDATE_FAILED.
const (
	lifespanFailureNotAuthorized   lifespanFailureClass = "not_authorized"
	lifespanFailureInvalidLifespan lifespanFailureClass = "invalid_lifespan"
	lifespanFailureReadOnly        lifespanFailureClass = "read_only"
	lifespanFailureOperationFailed lifespanFailureClass = "operation_failed"
)

// loginHintAttributeProperty is the identifying step's property naming the attribute a login_hint is
// matched against. A flow whose identifying step sets it can find a user who isn't there.
const loginHintAttributeProperty = "loginHintAttribute"

// onboardDetailTokenGuide is the section onboard_agent adds only when asked: how the agent gets each kind
// of token.
const onboardDetailTokenGuide = "token_guide"
