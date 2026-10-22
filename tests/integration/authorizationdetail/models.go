// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authorizationdetail holds integration tests for the RFC 9396 authorization details type registry.
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
