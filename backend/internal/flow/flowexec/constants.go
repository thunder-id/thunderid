/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package flowexec

const (
	defaultAuthFlowExpiry           int64 = 1800  // 30 minutes in seconds
	defaultRegistrationFlowExpiry   int64 = 3600  // 60 minutes in seconds
	defaultUserOnboardingFlowExpiry int64 = 86400 // 24 hours in seconds
	defaultRecoveryFlowExpiry       int64 = 1800  // 30 minutes in seconds

	fieldFlowSecret = "flowSecret"
)

// flowInitiationMode classifies how an application is permitted to initiate a new authentication
// flow directly over HTTP. It is derived at runtime from the application's inbound protocol
// configuration.
type flowInitiationMode int

const (
	// flowInitiationNotPermitted indicates the application may not initiate a new authentication flow
	// via a direct HTTP call. This covers redirect-based apps (OAuth 2.0 authorization_code grant),
	// which must initiate through their protocol component, and machine-to-machine apps
	// (client_credentials as the only grant), which obtain tokens directly at the token endpoint and
	// do not run flows. Neither is issued a Flow Secret.
	flowInitiationNotPermitted flowInitiationMode = iota
	// flowInitiationFlowSecret indicates a backend / server-side application — one that does not sign
	// in by redirect, or an embedded app with no protocol profile at all — that may initiate a flow
	// directly by presenting a valid Flow Secret.
	flowInitiationFlowSecret
	// flowInitiationAttestation indicates a mobile application that may initiate a flow directly by
	// presenting a valid platform attestation (e.g. a Google Play Integrity token) proving its binary
	// identity. This takes precedence over the redirect-based classification for apps that configure
	// attestation.
	flowInitiationAttestation
)
