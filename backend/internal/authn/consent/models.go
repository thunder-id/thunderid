// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// Wire-level discriminator values for ConsentPurposePrompt.Type. These are the strings the UI
// reads to choose between attribute, permission and authorization details rendering.
const (
	consentPromptTypeAttributes           = "attributes"
	consentPromptTypePermissions          = "permissions"
	consentPromptTypeAuthorizationDetails = "authorization_details"
)

// consentSessionData holds the consent session state that is signed into a JWT token.
// It captures the purposes and their elements from the resolve step so that the record step
// can verify that the user's decisions match exactly what was prompted.
type consentSessionData struct {
	// Purposes holds the per-purpose element information from the resolve step
	Purposes []consentSessionPurpose `json:"purposes"`
	// CoveredDetails are the requested authorization details earlier approvals cover, which are not
	// prompted and are granted as requested when the prompt is approved.
	CoveredDetails []providers.AuthorizationDetail `json:"coveredDetails,omitempty"`
}

// consentSessionPurpose represents a single purpose's elements within the consent session.
type consentSessionPurpose struct {
	// PurposeName is the unique name of the consent purpose
	PurposeName string `json:"purposeName"`
	// Essential holds the names of mandatory elements for this purpose
	Essential []string `json:"essential"`
	// Optional holds the names of optional elements for this purpose
	Optional []string `json:"optional"`
	// Details holds the authorization details prompted for, so the recorded values are the ones shown
	Details []providers.AuthorizationDetail `json:"details,omitempty"`
}
