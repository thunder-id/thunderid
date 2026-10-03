// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// cimdDocument holds the Client ID Metadata Document fields ThunderID reads.
type cimdDocument struct {
	ClientID                string          `json:"client_id"`
	ClientName              string          `json:"client_name"`
	ClientURI               string          `json:"client_uri"`
	TosURI                  string          `json:"tos_uri"`
	PolicyURI               string          `json:"policy_uri"`
	Contacts                []string        `json:"contacts"`
	RedirectURIs            []string        `json:"redirect_uris"`
	GrantTypes              []string        `json:"grant_types"`
	TokenEndpointAuthMethod string          `json:"token_endpoint_auth_method"`
	JWKSURI                 string          `json:"jwks_uri"`
	JWKS                    json.RawMessage `json:"jwks"`
	ClientSecret            string          `json:"client_secret"`
}

// PreviewRequest is the request body of a Client ID Metadata Document preview.
type PreviewRequest struct {
	ClientID string `json:"clientId"`
}

// PreviewResponse holds the values taken from a Client ID Metadata Document, in the shape of the
// fields a create request takes.
type PreviewResponse struct {
	Name              string                                  `json:"name"`
	URL               string                                  `json:"url,omitempty"`
	TosURI            string                                  `json:"tosUri,omitempty"`
	PolicyURI         string                                  `json:"policyUri,omitempty"`
	Contacts          []string                                `json:"contacts,omitempty"`
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig"`
}
