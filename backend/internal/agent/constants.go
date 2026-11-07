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

// Error codes the project service returns when it resolves the organization unit of a project. They
// are matched by value, since the Data Plane must not link the Control Plane only project package.
const (
	projectNotFoundErrorCode = "PRJ-1002"
	ouNotInProjectErrorCode  = "PRJ-1010"
)

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

// lifespanFailureClass is the failure class AGENT_LIFESPAN_UPDATE_FAILED reports.
type lifespanFailureClass string

// Failure classes reported by AGENT_LIFESPAN_UPDATE_FAILED.
const (
	lifespanFailureNotAuthorized   lifespanFailureClass = "not_authorized"
	lifespanFailureInvalidLifespan lifespanFailureClass = "invalid_lifespan"
	lifespanFailureReadOnly        lifespanFailureClass = "read_only"
	lifespanFailureOperationFailed lifespanFailureClass = "operation_failed"
)
