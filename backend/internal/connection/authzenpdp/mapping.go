// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import (
	"strings"
)

func fromRequest(req ConnectionRequest) AuthZENPDPConnection {
	connection := AuthZENPDPConnection{
		Name:                     req.Name,
		Description:              req.Description,
		Endpoint:                 req.Endpoint,
		BatchEndpoint:            req.BatchEndpoint,
		TimeoutMS:                req.TimeoutMS,
		RetryCount:               -1,
		SubjectAttributeMappings: sanitizeSubjectAttributeMappings(req.SubjectAttributeMappings),
	}
	if req.RetryCount != nil {
		connection.RetryCount = *req.RetryCount
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
		SubjectAttributeMappings: connection.SubjectAttributeMappings,
	}
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
