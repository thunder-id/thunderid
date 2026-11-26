// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmeta

import "encoding/json"

// declarativeDefaultReturnURL is the defaultReturnUrl the declarative flow section fixture seeds
// (resources/declarative_resources/server_configs/flow.yaml); it must match that file.
const declarativeDefaultReturnURL = "https://declarative-return.example.com"

type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// FlowMetadataResponse represents the aggregated metadata response from /flow/meta.
type FlowMetadataResponse struct {
	IsRegistrationFlowEnabled bool                 `json:"isRegistrationFlowEnabled"`
	Application               *ApplicationMetadata `json:"application,omitempty"`
	OU                        *OUMetadata          `json:"ou,omitempty"`
	Design                    DesignMetadata       `json:"design"`
	I18n                      I18nMetadata         `json:"i18n"`
	DefaultReturnURL          string               `json:"defaultReturnUrl,omitempty"`
}

// ApplicationMetadata represents application-specific metadata.
type ApplicationMetadata struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"`
	URL         string `json:"url,omitempty"`
	TosURI      string `json:"tosUri,omitempty"`
	PolicyURI   string `json:"policyUri,omitempty"`
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

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description,omitempty"`
}
