// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmeta

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/actorprovider"
)

// FlowMetadataResponse represents the aggregated metadata for a flow.
type FlowMetadataResponse struct {
	IsRegistrationFlowEnabled bool                               `json:"isRegistrationFlowEnabled"`
	IsRecoveryFlowEnabled     bool                               `json:"isRecoveryFlowEnabled"`
	Application               *actorprovider.ApplicationMetadata `json:"application,omitempty"`
	OU                        *OUMetadata                        `json:"ou,omitempty"`
	Design                    DesignMetadata                     `json:"design"`
	I18n                      I18nMetadata                       `json:"i18n"`
	DefaultReturnURL          string                             `json:"defaultReturnUrl,omitempty"`
}

// OUMetadata represents organization unit metadata.
type OUMetadata struct {
	ID              string `json:"id,omitempty"`
	Handle          string `json:"handle,omitempty"`
	Name            string `json:"name,omitempty"`
	Description     string `json:"description,omitempty"`
	LogoURL         string `json:"logoUrl,omitempty"`
	TosURI          string `json:"tosUri,omitempty"`
	PolicyURI       string `json:"policyUri,omitempty"`
	CookiePolicyURI string `json:"cookiePolicyUri,omitempty"`
}

// DesignMetadata represents theme and layout configuration.
type DesignMetadata struct {
	Theme  json.RawMessage `json:"theme"`
	Layout json.RawMessage `json:"layout"`
}

// I18nMetadata represents internationalization data.
type I18nMetadata struct {
	Languages    []string                     `json:"languages"`
	Language     string                       `json:"language"`
	TotalResults int                          `json:"totalResults"`
	Translations map[string]map[string]string `json:"translations"`
}
