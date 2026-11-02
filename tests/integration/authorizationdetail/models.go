// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authorizationdetail holds integration tests for RFC 9396 rich authorization requests: the
// authorization details type registry, its metadata, and details carried through token requests.
package authorizationdetail

import "encoding/json"

// processorAuthentication mirrors the outbound authentication of a type's processor.
type processorAuthentication struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// processorConfig mirrors the processor of an authorization details type.
type processorConfig struct {
	Endpoint       string                   `json:"endpoint"`
	Authentication *processorAuthentication `json:"authentication,omitempty"`
}

// typeRequest mirrors the request body that creates or updates an authorization details type.
type typeRequest struct {
	Type            string           `json:"type"`
	Name            string           `json:"name"`
	Description     string           `json:"description,omitempty"`
	Schema          json.RawMessage  `json:"schema"`
	ConsentTemplate string           `json:"consentTemplate,omitempty"`
	Processor       *processorConfig `json:"processor,omitempty"`
	Reuse           bool             `json:"reuse"`
}

// typeResponse mirrors an authorization details type as the management API returns it.
type typeResponse struct {
	ID              string           `json:"id"`
	Type            string           `json:"type"`
	Name            string           `json:"name"`
	Description     string           `json:"description,omitempty"`
	Version         int              `json:"version"`
	Schema          json.RawMessage  `json:"schema"`
	ConsentTemplate string           `json:"consentTemplate,omitempty"`
	Processor       *processorConfig `json:"processor,omitempty"`
	Reuse           bool             `json:"reuse"`
	IsReadOnly      bool             `json:"isReadOnly"`
}

// typeListResponse mirrors one page of authorization details types.
type typeListResponse struct {
	TotalResults int            `json:"totalResults"`
	StartIndex   int            `json:"startIndex"`
	Count        int            `json:"count"`
	Types        []typeResponse `json:"types"`
}

// errorResponse mirrors a management API error.
type errorResponse struct {
	Code string `json:"code"`
}

// oauthErrorResponse mirrors an OAuth error response.
type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// typeMetadata mirrors the client view of a type the metadata endpoint serves.
type typeMetadata struct {
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	Version       int             `json:"version"`
	Schema        json.RawMessage `json:"schema"`
	Modifications json.RawMessage `json:"modifications"`
}

// typeMetadataList mirrors the list the metadata endpoint serves.
type typeMetadataList struct {
	Types []typeMetadata `json:"types"`
}

// processRequest mirrors the call ThunderID makes to a type's processor.
type processRequest struct {
	Subject struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"subject"`
	Client struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"client"`
	Resource string `json:"resource"`
	Details  []struct {
		DetailID    string          `json:"detailId"`
		Type        string          `json:"type"`
		TypeVersion int             `json:"typeVersion"`
		Detail      json.RawMessage `json:"detail"`
	} `json:"details"`
}

// processResult mirrors the processor's answer on one detail.
type processResult struct {
	DetailID string                     `json:"detailId"`
	Decision bool                       `json:"decision"`
	Patch    map[string]json.RawMessage `json:"patch,omitempty"`
	Context  map[string]string          `json:"context,omitempty"`
}
