// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
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
