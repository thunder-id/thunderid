// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

const (
	defaultAuthFlowExpiry           int64 = 3600  // 60 minutes in seconds
	defaultRegistrationFlowExpiry   int64 = 3600  // 60 minutes in seconds
	defaultUserOnboardingFlowExpiry int64 = 86400 // 24 hours in seconds
	defaultSelfFlowExpiry           int64 = 1800  // 30 minutes in seconds
	defaultRecoveryFlowExpiry       int64 = 1800  // 30 minutes in seconds
	defaultSignOutFlowExpiry        int64 = 1800  // 30 minutes in seconds

	fieldFlowSecret = authnprovidercm.CredentialTypeFlowSecret

	// applicationTypePropertyKey is the InboundClient.Properties key under which the application
	// type is stored.
	applicationTypePropertyKey = "type"

	// maxNodeVisitsPerRequest is the number of times a node may run within a single request. A flow
	// that goes past it is cycling without pausing for user input.
	maxNodeVisitsPerRequest = 10
)

// flowInitiationMode classifies how an application is permitted to initiate a new authentication
// flow directly over HTTP. It is derived at runtime from the application's inbound protocol
// configuration.
type flowInitiationMode int

const (
	// flowInitiationNotPermitted indicates the application may not initiate a new authentication flow
	// via a direct HTTP call. This covers redirect-based apps (OAuth 2.0 authorization_code grant),
	// which must initiate through their protocol component, and apps without the token-exchange
	// grant (such as client_credentials, jwt-bearer, or CIBA clients), which cannot redeem a flow
	// assertion and do not run flows themselves. Neither is issued a Flow Secret.
	flowInitiationNotPermitted flowInitiationMode = iota
	// flowInitiationFlowSecret indicates a backend / server-side application — one that does not sign
	// in by redirect and has the token-exchange grant, or an embedded app with no protocol profile at
	// all — that may initiate a flow directly by presenting a valid Flow Secret.
	flowInitiationFlowSecret
	// flowInitiationAttestation indicates a mobile application that may initiate a flow directly by
	// presenting a valid platform attestation (e.g. a Google Play Integrity token) proving its binary
	// identity. This takes precedence over the redirect-based classification for apps that configure
	// attestation.
	flowInitiationAttestation
	// flowInitiationDevMode indicates a mobile application with attestation dev mode enabled, which may
	// initiate a flow directly without presenting a platform attestation. Intended for testing and
	// trying out sample or development mobile clients; disabled by default.
	flowInitiationDevMode
)

// onboardingFlow is a flow execute_flow starts.
type onboardingFlow string

// Flows execute_flow starts.
const (
	flowAgentOnboarding onboardingFlow = "agent_onboarding"
	flowUserOnboarding  onboardingFlow = "user_onboarding"
)

// onboardingFlows lists every flow execute_flow starts.
var onboardingFlows = []onboardingFlow{flowAgentOnboarding, flowUserOnboarding}

// flowRunStatus is the state execute_flow reports a run in.
type flowRunStatus string

// States execute_flow reports a run in.
const (
	flowStatusComplete   flowRunStatus = "complete"
	flowStatusNeedsInput flowRunStatus = "needs_input"
	flowStatusInfo       flowRunStatus = "info"
)

// execute_flow notices.
const (
	// secretNotice goes with a client secret a flow returns.
	secretNotice = "clientSecret is shown only this once, and no tool can return it again. Hand it to the " +
		"administrator to keep in a secret manager, and don't repeat it in the conversation."

	// infoNotice goes with a step that only shows values.
	infoNotice = "This step shows the values in data and asks for nothing. If it says someone else " +
		"finishes the flow, such as an invited user opening their link, nothing more is needed from you. " +
		"Otherwise call execute_flow with executionId to go on."
)
