// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package authzenpdp manages external AuthZEN PDP connections.
package authzenpdp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// VendorName is the connection vendor identifier for external AuthZEN PDPs.
const VendorName = "external-authzen-pdp"

const (
	// DefaultTimeoutMS is the default external PDP request timeout in milliseconds.
	DefaultTimeoutMS = 500
	// DefaultRetryCount is the default number of retries for external PDP requests.
	DefaultRetryCount = 1
)

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
	FailOpen                 bool                      `json:"failOpen,omitempty"`
}

// UnmarshalJSON decodes an external AuthZEN PDP connection request.
func (r *ConnectionRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name                     string                    `json:"name"`
		Description              string                    `json:"description,omitempty"`
		Endpoint                 json.RawMessage           `json:"endpoint"`
		BatchEndpoint            string                    `json:"batchEndpoint,omitempty"`
		TimeoutMS                int                       `json:"timeoutMs,omitempty"`
		RetryCount               int                       `json:"retryCount,omitempty"`
		SubjectProperties        string                    `json:"subjectProperties,omitempty"`
		SubjectPropertyMappings  string                    `json:"subjectPropertyMappings,omitempty"`
		SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
		FailOpen                 bool                      `json:"failOpen,omitempty"`
	}
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
	r.FailOpen = raw.FailOpen
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
		RetryCount               int                       `json:"retryCount,omitempty"`
		SubjectProperties        string                    `json:"subjectProperties,omitempty"`
		SubjectPropertyMappings  string                    `json:"subjectPropertyMappings,omitempty"`
		SubjectAttributeMappings []SubjectAttributeMapping `json:"subjectAttributeMappings,omitempty"`
		FailOpen                 bool                      `json:"failOpen,omitempty"`
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
		FailOpen:                 r.FailOpen,
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
	FailOpen                 bool                      `json:"failOpen,omitempty"`
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
	FailOpen                 bool
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
	FailOpen                 bool
}

// ListAuthZENPDPRuntimeConfigs returns saved AuthZEN PDP connections for token-issuance routing.
func ListAuthZENPDPRuntimeConfigs(ctx context.Context) ([]AuthZENPDPRuntimeConfig, error) {
	connections, err := NewStore().List(ctx)
	if err != nil {
		return nil, err
	}
	configs := make([]AuthZENPDPRuntimeConfig, 0, len(connections))
	for _, connection := range connections {
		configs = append(configs, AuthZENPDPRuntimeConfig{
			ID:                       connection.ID,
			Name:                     connection.Name,
			Endpoint:                 strings.TrimSpace(connection.Endpoint),
			BatchEndpoint:            connection.BatchEndpoint,
			TimeoutMS:                connection.TimeoutMS,
			RetryCount:               connection.RetryCount,
			SubjectProperties:        append([]string(nil), connection.SubjectProperties...),
			SubjectPropertyMappings:  cloneStringMap(connection.SubjectPropertyMappings),
			SubjectAttributeMappings: cloneSubjectAttributeMappings(connection.SubjectAttributeMappings),
			FailOpen:                 connection.FailOpen,
		})
	}
	return configs, nil
}

// GetAuthZENPDPRuntimeConfig returns a saved AuthZEN PDP connection for token-issuance routing.
func GetAuthZENPDPRuntimeConfig(ctx context.Context, id string) (*AuthZENPDPRuntimeConfig, error) {
	connection, err := NewStore().Get(ctx, id)
	if err != nil || connection == nil {
		return nil, err
	}
	return &AuthZENPDPRuntimeConfig{
		ID:                       connection.ID,
		Name:                     connection.Name,
		Endpoint:                 strings.TrimSpace(connection.Endpoint),
		BatchEndpoint:            connection.BatchEndpoint,
		TimeoutMS:                connection.TimeoutMS,
		RetryCount:               connection.RetryCount,
		SubjectProperties:        append([]string(nil), connection.SubjectProperties...),
		SubjectPropertyMappings:  cloneStringMap(connection.SubjectPropertyMappings),
		SubjectAttributeMappings: cloneSubjectAttributeMappings(connection.SubjectAttributeMappings),
		FailOpen:                 connection.FailOpen,
	}, nil
}

// FromRequest converts an API request into an internal connection.
func FromRequest(req ConnectionRequest) AuthZENPDPConnection {
	connection := AuthZENPDPConnection{
		Name:                     req.Name,
		Description:              req.Description,
		Endpoint:                 req.Endpoint,
		BatchEndpoint:            req.BatchEndpoint,
		TimeoutMS:                req.TimeoutMS,
		RetryCount:               req.RetryCount,
		SubjectProperties:        splitSubjectProperties(req.SubjectProperties),
		SubjectPropertyMappings:  SplitSubjectPropertyMappings(req.SubjectPropertyMappings),
		SubjectAttributeMappings: sanitizeSubjectAttributeMappings(req.SubjectAttributeMappings),
		FailOpen:                 req.FailOpen,
	}
	if connection.TimeoutMS <= 0 {
		connection.TimeoutMS = DefaultTimeoutMS
	}
	if connection.RetryCount <= 0 {
		connection.RetryCount = DefaultRetryCount
	}
	return connection
}

// ToResponse converts an internal connection into an API response.
func ToResponse(connection AuthZENPDPConnection) ConnectionResponse {
	return ConnectionResponse{
		ID:                       connection.ID,
		Name:                     connection.Name,
		Description:              connection.Description,
		Type:                     VendorName,
		Endpoint:                 connection.Endpoint,
		BatchEndpoint:            connection.BatchEndpoint,
		TimeoutMS:                connection.TimeoutMS,
		RetryCount:               connection.RetryCount,
		SubjectProperties:        strings.Join(connection.SubjectProperties, " "),
		SubjectPropertyMappings:  JoinSubjectPropertyMappings(connection.SubjectPropertyMappings),
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
		FailOpen:                 connection.FailOpen,
	}
}

// SplitSubjectPropertyMappings parses the API representation of subject-property mappings.
func SplitSubjectPropertyMappings(value string) map[string]string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	mappings := map[string]string{}
	for _, segment := range strings.Split(value, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		separator := strings.Index(segment, ":")
		if separator <= 0 {
			continue
		}
		source := strings.TrimSpace(segment[:separator])
		target := strings.TrimSpace(segment[separator+1:])
		if source != "" && target != "" {
			mappings[source] = target
		}
	}
	if len(mappings) == 0 {
		return nil
	}
	return mappings
}

// JoinSubjectPropertyMappings serializes subject-property mappings for the API representation.
func JoinSubjectPropertyMappings(mappings map[string]string) string {
	if len(mappings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(mappings))
	for source, target := range mappings {
		if source != "" && target != "" {
			parts = append(parts, source+": "+target)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneSubjectAttributeMappings(groups []SubjectAttributeMapping) []SubjectAttributeMapping {
	if len(groups) == 0 {
		return nil
	}
	clone := make([]SubjectAttributeMapping, 0, len(groups))
	for _, group := range groups {
		clone = append(clone, SubjectAttributeMapping{
			UserType:   group.UserType,
			Attributes: append([]SubjectAttributeRow(nil), group.Attributes...),
		})
	}
	return clone
}

func splitSubjectProperties(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
	properties := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		property := strings.TrimSpace(part)
		if property == "" {
			continue
		}
		if _, ok := seen[property]; ok {
			continue
		}
		seen[property] = struct{}{}
		properties = append(properties, property)
	}
	return properties
}

func sanitizeSubjectAttributeMappings(groups []SubjectAttributeMapping) []SubjectAttributeMapping {
	if len(groups) == 0 {
		return nil
	}
	result := make([]SubjectAttributeMapping, 0, len(groups))
	for _, group := range groups {
		attributes := make([]SubjectAttributeRow, 0, len(group.Attributes))
		for _, attribute := range group.Attributes {
			name := strings.TrimSpace(attribute.Attribute)
			if name == "" {
				continue
			}
			attributes = append(attributes, SubjectAttributeRow{
				Attribute:    name,
				PDPAttribute: strings.TrimSpace(attribute.PDPAttribute),
			})
		}
		userType := strings.TrimSpace(group.UserType)
		if userType == "" && len(attributes) == 0 {
			continue
		}
		result = append(result, SubjectAttributeMapping{
			UserType:   userType,
			Attributes: attributes,
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// NormalizedSubjectMapping returns direct subject-property mappings.
func NormalizedSubjectMapping(connection AuthZENPDPConnection) ([]string, map[string]string) {
	subjectProperties := append([]string(nil), connection.SubjectProperties...)
	subjectPropertyMappings := cloneStringMap(connection.SubjectPropertyMappings)
	return subjectProperties, subjectPropertyMappings
}
