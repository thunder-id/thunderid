// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

// VendorName is the connection vendor identifier for AuthZEN PDPs.
const VendorName = "authzen-pdp"

// ConnectionRequest is the API representation of an AuthZEN PDP connection request.
type ConnectionRequest struct {
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                       `json:"timeoutMs,omitempty"`
	RetryCount               *int                      `json:"retryCount,omitempty"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

// ConnectionResponse is the API representation of an AuthZEN PDP connection.
type ConnectionResponse struct {
	ID                       string                    `json:"id"`
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Type                     string                    `json:"type"`
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                       `json:"timeoutMs"`
	RetryCount               int                       `json:"retryCount"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

// AuthZENPDPConnection is the internal representation of an AuthZEN PDP connection.
type AuthZENPDPConnection struct {
	ID                       string
	Name                     string
	Description              string
	Endpoint                 string
	BatchEndpoint            string
	TimeoutMS                int
	RetryCount               int
	SubjectAttributeMappings []SubjectAttributeMapping
}

// SubjectAttributeMapping maps ThunderID user-type attributes to PDP subject attributes.
type SubjectAttributeMapping struct {
	UserType   string                `json:"userType" yaml:"userType"`
	Attributes []SubjectAttributeRow `json:"attributes" yaml:"attributes"`
}

// SubjectAttributeRow identifies one subject attribute mapping.
type SubjectAttributeRow struct {
	Attribute    string `json:"attribute" yaml:"attribute"`
	PDPAttribute string `json:"pdpAttribute,omitempty" yaml:"pdpAttribute,omitempty"`
}
