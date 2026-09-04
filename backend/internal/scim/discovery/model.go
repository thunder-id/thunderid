// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// scimSupportedFeature captures a simple supported/unsupported capability flag.
type scimSupportedFeature struct {
	Supported bool `json:"supported"`
}

// scimBulkConfig captures bulk operation capability flags.
type scimBulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

// scimFilterConfig captures filter capability flags.
type scimFilterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

// scimAuthenticationScheme describes one supported authentication mechanism.
type scimAuthenticationScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// scimPaginationConfig captures pagination capability flags per RFC 9865.
type scimPaginationConfig struct {
	Cursor                  bool   `json:"cursor"`
	Index                   bool   `json:"index"`
	DefaultPaginationMethod string `json:"defaultPaginationMethod,omitempty"`
	DefaultPageSize         int    `json:"defaultPageSize,omitempty"`
	MaxPageSize             int    `json:"maxPageSize,omitempty"`
}

// SCIMServiceProviderConfig is the response body for GET /scim/v2/ServiceProviderConfig.
type SCIMServiceProviderConfig struct {
	Schemas               []string                   `json:"schemas"`
	Patch                 scimSupportedFeature       `json:"patch"`
	Bulk                  scimBulkConfig             `json:"bulk"`
	Filter                scimFilterConfig           `json:"filter"`
	ChangePassword        scimSupportedFeature       `json:"changePassword"`
	Sort                  scimSupportedFeature       `json:"sort"`
	ETag                  scimSupportedFeature       `json:"etag"`
	Pagination            scimPaginationConfig       `json:"pagination"`
	AuthenticationSchemes []scimAuthenticationScheme `json:"authenticationSchemes"`
	Meta                  scim.SCIMMeta              `json:"meta"`
}

// scimSchemaAttribute represents a single attribute definition within a SCIM Schema resource.
// RFC 7643 §7
type scimSchemaAttribute struct {
	Name            string                `json:"name"`
	Type            scimAttrType          `json:"type"`
	MultiValued     bool                  `json:"multiValued"`
	Description     string                `json:"description,omitempty"`
	Required        bool                  `json:"required"`
	CaseExact       bool                  `json:"caseExact"`
	Mutability      scimMutability        `json:"mutability"`
	Returned        scimReturned          `json:"returned"`
	Uniqueness      scimUniqueness        `json:"uniqueness"`
	CanonicalValues []string              `json:"canonicalValues,omitempty"`
	SubAttributes   []scimSchemaAttribute `json:"subAttributes,omitempty"`
}

// SCIMSchema is the response body for a single SCIM Schema resource.
// RFC 7643 §7
type SCIMSchema struct {
	Schemas     []string              `json:"schemas,omitempty"`
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Attributes  []scimSchemaAttribute `json:"attributes"`
	Meta        scim.SCIMMeta         `json:"meta"`
}

// SCIMSchemaListResponse is the SCIM ListResponse envelope for Schema resources.
// RFC 7644 §3.4.2
type SCIMSchemaListResponse struct {
	Schemas      []string     `json:"schemas"`
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	ItemsPerPage int          `json:"itemsPerPage"`
	Resources    []SCIMSchema `json:"Resources"`
}

// scimResourceTypeSchemaExtension represents a schema extension entry within a
// ResourceType resource. RFC 7643 §6.
type scimResourceTypeSchemaExtension struct {
	Schema   string `json:"schema"`
	Required bool   `json:"required"`
}

// SCIMResourceType is the response body for a single SCIM ResourceType resource.
// RFC 7643 §6.
type SCIMResourceType struct {
	Schemas          []string                          `json:"schemas"`
	ID               string                            `json:"id"`
	Name             string                            `json:"name"`
	Description      string                            `json:"description,omitempty"`
	Endpoint         string                            `json:"endpoint"`
	Schema           string                            `json:"schema"`
	SchemaExtensions []scimResourceTypeSchemaExtension `json:"schemaExtensions"`
	Meta             scim.SCIMMeta                     `json:"meta"`
}

// SCIMResourceTypeListResponse is the SCIM ListResponse envelope for ResourceType resources.
// RFC 7644 §3.4.2
type SCIMResourceTypeListResponse struct {
	Schemas      []string           `json:"schemas"`
	TotalResults int                `json:"totalResults"`
	StartIndex   int                `json:"startIndex"`
	ItemsPerPage int                `json:"itemsPerPage"`
	Resources    []SCIMResourceType `json:"Resources"`
}
