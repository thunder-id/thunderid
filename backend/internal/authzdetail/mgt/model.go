// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// typeProperties is the JSON structure of the AUTHORIZATION_DETAIL_TYPE.PROPERTIES column.
type typeProperties struct {
	ConsentTemplate string           `json:"consentTemplate,omitempty"`
	Processor       *storedProcessor `json:"processor,omitempty"`
	Reuse           *bool            `json:"reuse,omitempty"`
}

// storedProcessor is a type's processor as stored, with encrypted authentication properties.
type storedProcessor struct {
	Endpoint                 string          `json:"endpoint"`
	AuthenticationProperties json.RawMessage `json:"authenticationProperties,omitempty"`
}

// ProcessorRequest is the processor of a type in a management API request.
type ProcessorRequest struct {
	Endpoint       string                       `json:"endpoint"`
	Authentication *outboundauth.Authentication `json:"authentication,omitempty"`
}

// ProcessorResponse is the processor of a type in a management API response, with credentials masked.
type ProcessorResponse struct {
	Endpoint       string                      `json:"endpoint"`
	Authentication outboundauth.Authentication `json:"authentication"`
}

// AuthorizationDetailTypeRequest is the request body for creating or updating a type.
type AuthorizationDetailTypeRequest struct {
	Type            string            `json:"type"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	Schema          json.RawMessage   `json:"schema"`
	ConsentTemplate string            `json:"consentTemplate,omitempty"`
	Processor       *ProcessorRequest `json:"processor,omitempty"`
	Reuse           *bool             `json:"reuse,omitempty"`
}

// typeRequestWire is the management API request body. It also accepts the fields a GET returns, so a
// fetched type can be sent back, and ignores them.
type typeRequestWire struct {
	AuthorizationDetailTypeRequest
	ID         string `json:"id,omitempty"`
	Version    int    `json:"version,omitempty"`
	IsReadOnly bool   `json:"isReadOnly,omitempty"`
}

// AuthorizationDetailTypeResponse is the response body describing a type.
type AuthorizationDetailTypeResponse struct {
	ID              string             `json:"id"`
	Type            string             `json:"type"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	Version         int                `json:"version"`
	Schema          json.RawMessage    `json:"schema"`
	ConsentTemplate string             `json:"consentTemplate,omitempty"`
	Processor       *ProcessorResponse `json:"processor,omitempty"`
	Reuse           bool               `json:"reuse"`
	IsReadOnly      bool               `json:"isReadOnly"`
}

// AuthorizationDetailTypePage is one page of the registered types.
type AuthorizationDetailTypePage struct {
	TotalResults int
	Types        []providers.AuthorizationDetailType
}

// AuthorizationDetailTypeListResponse is the response body listing the registered types.
type AuthorizationDetailTypeListResponse struct {
	TotalResults int                               `json:"totalResults"`
	StartIndex   int                               `json:"startIndex"`
	Count        int                               `json:"count"`
	Types        []AuthorizationDetailTypeResponse `json:"types"`
	Links        []sysutils.Link                   `json:"links"`
}
