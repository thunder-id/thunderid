// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

// ConnectionRequest is the API representation of an AuthZEN PDP connection request.
type ConnectionRequest struct {
	ID                       string                    `json:"-" yaml:"-"`
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

// ToResponse converts an internal connection into an API response.
func ToResponse(connection AuthZENPDPConnection) ConnectionResponse {
	return ConnectionResponse{
		ID:                       connection.ID,
		Name:                     connection.Name,
		Description:              connection.Description,
		Type:                     "authzen-pdp",
		Endpoint:                 connection.Endpoint,
		BatchEndpoint:            connection.BatchEndpoint,
		TimeoutMS:                connection.TimeoutMS,
		RetryCount:               connection.RetryCount,
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	}
}

// AuthZENPDPConnection is the internal representation of an AuthZEN PDP connection.
type AuthZENPDPConnection struct {
	ID                       string
	Name                     string
	Description              string
	IsReadOnly               bool
	Endpoint                 string
	BatchEndpoint            string
	TimeoutMS                int
	RetryCount               int
	SubjectAttributeMappings []SubjectAttributeMapping
}

// SubjectAttributeMapping maps attributes from a ThunderID entity type to PDP subject attributes.
type SubjectAttributeMapping struct {
	EntityType string                `json:"entityType" yaml:"entityType"`
	Attributes []SubjectAttributeRow `json:"attributes" yaml:"attributes"`
}

// SubjectAttributeRow identifies one subject attribute mapping.
type SubjectAttributeRow struct {
	Attribute    string `json:"attribute" yaml:"attribute"`
	PDPAttribute string `json:"pdpAttribute,omitempty" yaml:"pdpAttribute,omitempty"`
}
