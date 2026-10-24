// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

const (
	// defaultPageSize is the default number of items per page for paginated responses
	defaultPageSize = 30
	// maxPageSize is the maximum number of items per page for paginated responses
	maxPageSize = 100
	// maxAllowedVersionHistory is the maximum number of versions to keep for a flow definition
	maxAllowedVersionHistory = 50
	// defaultVersionHistory is the default number of versions to keep for a flow definition
	defaultVersionHistory = 10
)

const (
	// provisioningNodeID is the node ID for the inferred provisioning node
	provisioningNodeID = "prov_node"
	// userTypeResolverNodeID is the node ID for the inferred user type resolver node
	userTypeResolverNodeID = "ut_res_node"
	// userTypePromptNodeID is the node ID for the inferred user type prompt node
	userTypePromptNodeID = "ut_prompt_node"
	// phoneInputPromptNodeID is the node ID for the inferred phone input prompt node
	phoneInputPromptNodeID = "phone_prompt_node"
	// defaultNodeWidth is the default width for a node layout
	defaultNodeWidth = 100
	// defaultNodeHeight is the default height for a node layout
	defaultNodeHeight = 120
	// defaultNodeXPos is the default X position for a node layout
	defaultNodeXPos = 0
	// defaultNodeYPos is the default Y position for a node layout
	defaultNodeYPos = 0
)

const (
	// nodePropertyKeyIDPID is the node property key that references an identity provider by its ID.
	nodePropertyKeyIDPID = "idpId"
	// nodePropertyKeyNotificationSenderID is the node property key that references a notification
	// sender by its ID.
	nodePropertyKeyNotificationSenderID = "senderId"
	// nodePropertyKeyEmailTemplate / nodePropertyKeySMSTemplate reference a managed notification
	// template by its handle; the key implies the channel. They mirror the executor property keys
	// (internal/flow/executor).
	nodePropertyKeyEmailTemplate = "emailTemplate"
	nodePropertyKeySMSTemplate   = "smsTemplate"
)

// authToRegLabelTerms maps authentication UI label terms to their registration equivalents.
// Ordered by specificity (longest/most-specific first) to avoid partial matches.
var authToRegLabelTerms = []struct{ auth, reg string }{
	{"Authentication", "Registration"},
	{"Authenticate", "Register"},
	{"Sign-in", "Sign-up"},
	{"Sign In", "Sign Up"},
	{"Signin", "Signup"},
	{"Log In", "Register"},
	{"Login", "Register"},
}
