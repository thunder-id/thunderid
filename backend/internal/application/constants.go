// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

// Field keys for entity system attributes.
const (
	fieldName         = "name"
	fieldDescription  = "description"
	fieldClientID     = "clientId"
	fieldClientSecret = authnprovidercm.CredentialTypeClientSecret
	fieldFlowSecret   = authnprovidercm.CredentialTypeFlowSecret
)

// Field keys for application config properties.
const (
	propURL         = "url"
	propLogoURL     = "logo_url"
	propTosURI      = "tos_uri"
	propPolicyURI   = "policy_uri"
	propContacts    = "contacts"
	propType        = "type"
	propTemplate    = "template"
	propMetadata    = "metadata"
	propOAuthConfig = "oauth_config"
)

// projectIDQueryParam is the query parameter that lists the applications of one project.
const projectIDQueryParam = "projectId"

// Error codes the project service returns when it resolves the organization unit of a project. They
// are matched by value, since the Data Plane must not link the Control Plane only project package.
const (
	projectNotFoundErrorCode = "PRJ-1002"
	ouNotInProjectErrorCode  = "PRJ-1010"
)

// The system flows this service runs, by handle, before a change is written and after, and the input
// naming the application they act on.
const (
	systemFlowApplicationDeletePre       = "system-application-delete-pre"
	systemFlowApplicationSecretRotatePre = "system-application-secret-rotate-pre"
	// systemFlowRevocationPost follows every change whose pre flow revokes up to a cutoff, moving the
	// cutoff past the write.
	systemFlowRevocationPost   = "system-revocation-post"
	systemFlowInputApplication = "targetApplicationId"
)

// clientSecretDisposition is what an update does with the application's client secret.
type clientSecretDisposition int

const (
	// clientSecretUnchanged: no secret was supplied, the client has no use for one, or the one
	// supplied is the one the application already has.
	clientSecretUnchanged clientSecretDisposition = iota
	// clientSecretSet: the application had no secret and gets one, so nothing was issued under one.
	clientSecretSet
	// clientSecretRotated: the supplied secret replaces the one the application has.
	clientSecretRotated
)
