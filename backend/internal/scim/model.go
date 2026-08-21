// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

// SCIMSupportedFeature captures a simple supported/unsupported capability flag.
type SCIMSupportedFeature struct {
	Supported bool `json:"supported"`
}

// SCIMBulkConfig captures bulk operation capability flags.
type SCIMBulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

// SCIMFilterConfig captures filter capability flags.
type SCIMFilterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

// SCIMAuthenticationScheme describes one supported authentication mechanism.
type SCIMAuthenticationScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SCIMMeta holds SCIM resource metadata fields.
type SCIMMeta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Location     string `json:"location,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Created      string `json:"created,omitempty"`
	Version      string `json:"version,omitempty"`
}

// SCIMPaginationConfig captures pagination capability flags per RFC 9865.
type SCIMPaginationConfig struct {
	Cursor                  bool   `json:"cursor"`
	Index                   bool   `json:"index"`
	DefaultPaginationMethod string `json:"defaultPaginationMethod,omitempty"`
	DefaultPageSize         int    `json:"defaultPageSize,omitempty"`
	MaxPageSize             int    `json:"maxPageSize,omitempty"`
}

// SCIMServiceProviderConfig is the response body for GET /scim/v2/ServiceProviderConfig.
type SCIMServiceProviderConfig struct {
	Schemas               []string                   `json:"schemas"`
	Patch                 SCIMSupportedFeature       `json:"patch"`
	Bulk                  SCIMBulkConfig             `json:"bulk"`
	Filter                SCIMFilterConfig           `json:"filter"`
	ChangePassword        SCIMSupportedFeature       `json:"changePassword"`
	Sort                  SCIMSupportedFeature       `json:"sort"`
	ETag                  SCIMSupportedFeature       `json:"etag"`
	Pagination            SCIMPaginationConfig       `json:"pagination"`
	AuthenticationSchemes []SCIMAuthenticationScheme `json:"authenticationSchemes"`
	Meta                  SCIMMeta                   `json:"meta"`
}

// SCIMSchemaAttribute represents a single attribute definition within a SCIM Schema resource.
// RFC 7643 §7
type SCIMSchemaAttribute struct {
	Name            string                `json:"name"`
	Type            SCIMAttrType          `json:"type"`
	MultiValued     bool                  `json:"multiValued"`
	Description     string                `json:"description,omitempty"`
	Required        bool                  `json:"required"`
	CaseExact       bool                  `json:"caseExact"`
	Mutability      SCIMMutability        `json:"mutability"`
	Returned        SCIMReturned          `json:"returned"`
	Uniqueness      SCIMUniqueness        `json:"uniqueness"`
	CanonicalValues []string              `json:"canonicalValues,omitempty"`
	SubAttributes   []SCIMSchemaAttribute `json:"subAttributes,omitempty"`
}

// SCIMSchema is the response body for a single SCIM Schema resource.
// RFC 7643 §7
type SCIMSchema struct {
	Schemas     []string              `json:"schemas,omitempty"`
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Attributes  []SCIMSchemaAttribute `json:"attributes"`
	Meta        SCIMMeta              `json:"meta"`
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

// SCIMResourceTypeSchemaExtension represents a schema extension entry within a
// ResourceType resource. RFC 7643 §6.
type SCIMResourceTypeSchemaExtension struct {
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
	SchemaExtensions []SCIMResourceTypeSchemaExtension `json:"schemaExtensions"`
	Meta             SCIMMeta                          `json:"meta"`
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

// SCIMErrorResponse is the SCIM-standard error payload shape (RFC 7643 §3.12).
// This is what goes over the wire to SCIM clients — never internal error codes.
type SCIMErrorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// SCIMSearchRequest is the request payload shape of the .search request
type SCIMSearchRequest struct {
	Schemas            []string `json:"schemas"`
	Filter             string   `json:"filter,omitempty"`
	Attributes         []string `json:"attributes,omitempty"`
	ExcludedAttributes []string `json:"excludedAttributes,omitempty"`
	SortBy             string   `json:"sortBy,omitempty"`
	SortOrder          string   `json:"sortOrder,omitempty"`
	StartIndex         int      `json:"startIndex,omitempty"`
	Count              *int     `json:"count,omitempty"`
}
