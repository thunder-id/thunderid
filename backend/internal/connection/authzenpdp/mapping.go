// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"sort"
	"strings"
)

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
	}
	if connection.TimeoutMS <= 0 {
		connection.TimeoutMS = DefaultTimeoutMS()
	}
	if connection.RetryCount < 0 {
		connection.RetryCount = DefaultRetryCount()
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

// cloneStringMap returns a shallow copy of a string map.
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

// cloneSubjectAttributeMappings copies subject mappings and their attribute rows.
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

// splitSubjectProperties parses and deduplicates a list of subject properties.
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

// sanitizeSubjectAttributeMappings trims mappings and removes empty entries.
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
