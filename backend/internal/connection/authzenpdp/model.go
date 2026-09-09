// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authzenpdp manages external AuthZEN PDP connections.
package authzenpdp

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// VendorName is the connection vendor identifier for external AuthZEN PDPs.
const VendorName = "authzen-pdp"

// DefaultTimeoutMS returns the configured server timeout default.
func DefaultTimeoutMS() int {
	if !config.IsServerRuntimeInitialized() {
		return 0
	}
	return config.GetServerRuntime().Config.AuthZENPDP.TimeoutMS
}

// DefaultRetryCount returns the configured server retry default.
func DefaultRetryCount() int {
	if !config.IsServerRuntimeInitialized() {
		return 0
	}
	value := config.GetServerRuntime().Config.AuthZENPDP.RetryCount
	if value == nil {
		return 0
	}
	return *value
}

// ConnectionRequest is the API representation of an external AuthZEN PDP connection request.
type ConnectionRequest struct {
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Endpoint                 string                    `json:"-"`
	BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                       `json:"timeoutMs,omitempty"`
	RetryCount               int                       `json:"retryCount,omitempty"`
	SubjectProperties        string                    `json:"subjectProperties,omitempty"`
	SubjectPropertyMappings  string                    `json:"subjectPropertyMappings,omitempty"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

// UnmarshalJSON decodes an external AuthZEN PDP connection request.
func (r *ConnectionRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name                     string                    `json:"name"`
		Description              string                    `json:"description,omitempty"`
		Endpoint                 json.RawMessage           `json:"endpoint"`
		BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
		TimeoutMS                int                       `json:"timeoutMs,omitempty"`
		RetryCount               int                       `json:"retryCount"`
		SubjectProperties        string                    `json:"subjectProperties,omitempty"`
		SubjectPropertyMappings  string                    `json:"subjectPropertyMappings,omitempty"`
		SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
	}
	raw.TimeoutMS = DefaultTimeoutMS()
	raw.RetryCount = DefaultRetryCount()
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Endpoint) > 0 && string(raw.Endpoint) != "null" {
		if err := json.Unmarshal(raw.Endpoint, &r.Endpoint); err != nil {
			return err
		}
	}
	r.Name = raw.Name
	r.Description = raw.Description
	r.TimeoutMS = raw.TimeoutMS
	r.RetryCount = raw.RetryCount
	r.SubjectProperties = raw.SubjectProperties
	r.SubjectPropertyMappings = raw.SubjectPropertyMappings
	r.SubjectAttributeMappings = raw.SubjectAttributeMappings
	r.BatchEndpoint = raw.BatchEndpoint
	return nil
}

// MarshalJSON encodes an external AuthZEN PDP connection request.
func (r ConnectionRequest) MarshalJSON() ([]byte, error) {
	payload := struct {
		Name                     string                    `json:"name"`
		Description              string                    `json:"description,omitempty"`
		Endpoint                 string                    `json:"endpoint"`
		BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
		TimeoutMS                int                       `json:"timeoutMs,omitempty"`
		RetryCount               int                       `json:"retryCount"`
		SubjectProperties        string                    `json:"subjectProperties,omitempty"`
		SubjectPropertyMappings  string                    `json:"subjectPropertyMappings,omitempty"`
		SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
	}{
		Name:                     r.Name,
		Description:              r.Description,
		Endpoint:                 r.Endpoint,
		BatchEndpoint:            r.BatchEndpoint,
		TimeoutMS:                r.TimeoutMS,
		RetryCount:               r.RetryCount,
		SubjectProperties:        r.SubjectProperties,
		SubjectPropertyMappings:  r.SubjectPropertyMappings,
		SubjectAttributeMappings: r.SubjectAttributeMappings,
	}
	return json.Marshal(payload)
}

// ConnectionResponse is the API representation of an external AuthZEN PDP connection.
type ConnectionResponse struct {
	ID                       string                    `json:"id"`
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Type                     string                    `json:"type"`
	Endpoint                 string                    `json:"endpoint"`
	BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
	TimeoutMS                int                       `json:"timeoutMs"`
	RetryCount               int                       `json:"retryCount"`
	SubjectProperties        string                    `json:"subjectProperties,omitempty"`
	SubjectPropertyMappings  string                    `json:"subjectPropertyMappings,omitempty"`
	SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
}

// AuthZENPDPConnection is the internal representation of an external AuthZEN PDP connection.
type AuthZENPDPConnection struct {
	ID                       string
	Name                     string
	Description              string
	Endpoint                 string
	BatchEndpoint            string
	TimeoutMS                int
	RetryCount               int
	SubjectProperties        []string
	SubjectPropertyMappings  map[string]string
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

// AuthZENPDPRuntimeConfig is the runtime-safe subset of a saved AuthZEN PDP connection.
type AuthZENPDPRuntimeConfig struct {
	ID                       string
	Name                     string
	Endpoint                 string
	BatchEndpoint            string
	TimeoutMS                int
	RetryCount               int
	SubjectProperties        []string
	SubjectPropertyMappings  map[string]string
	SubjectAttributeMappings []SubjectAttributeMapping
}
