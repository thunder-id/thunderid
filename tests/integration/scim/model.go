/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package scim

import "github.com/thunder-id/thunderid/tests/integration/testutils"

// scimBaseURL is the SCIM v2 API root, mounted on the same server as every
// other integration-tested endpoint (backend/internal/scim, SCIMBasePath).
const scimBaseURL = testutils.TestServerURL + "/scim/v2"

const (
	scimCoreUserSchemaURN  = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimCoreGroupSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:Group"
	scimPatchOpSchemaURN   = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	scimSearchSchemaURN    = "urn:ietf:params:scim:api:messages:2.0:SearchRequest"
)

// scimMeta mirrors backend/internal/scim's SCIMMeta wire shape.
type scimMeta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Location     string `json:"location,omitempty"`
	Version      string `json:"version,omitempty"`
}

// scimErrorResponse mirrors SCIMErrorResponse (RFC 7644 §3.12).
type scimErrorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// scimServiceProviderConfig is a partial mirror of SCIMServiceProviderConfig,
// only the fields these tests assert on.
type scimServiceProviderConfig struct {
	Patch      scimSupportedFeature   `json:"patch"`
	Filter     scimSupportedFeature   `json:"filter"`
	ETag       scimSupportedFeature   `json:"etag"`
	Pagination scimPaginationConfig   `json:"pagination"`
}

type scimSupportedFeature struct {
	Supported bool `json:"supported"`
}

// scimPaginationConfig mirrors the RFC 9865 pagination block of SCIMPaginationConfig.
type scimPaginationConfig struct {
	Cursor      bool `json:"cursor"`
	Index       bool `json:"index"`
	MaxPageSize int  `json:"maxPageSize"`
}

// scimResourceType mirrors the fields of SCIMResourceType these tests need.
type scimResourceType struct {
	ID       string `json:"id"`
	Endpoint string `json:"endpoint"`
}

type scimResourceTypeListResponse struct {
	TotalResults int                `json:"totalResults"`
	Resources    []scimResourceType `json:"Resources"`
}

// scimSchemaAttribute mirrors SCIMSchemaAttribute, only the fields used here.
type scimSchemaAttribute struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// scimSchema mirrors SCIMSchema. ID is the schema URN per RFC 7643 §7.
type scimSchema struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Attributes []scimSchemaAttribute `json:"attributes"`
}

type scimSchemaListResponse struct {
	TotalResults int          `json:"totalResults"`
	Resources    []scimSchema `json:"Resources"`
}

// scimUser mirrors the fixed fields of SCIMUser. There is deliberately no
// userName field here: ThunderID's SCIMUser has no static field for it either
// — core-mapped attributes like userName or emails are merged flat into the
// response only if the usertype schema declares a matching property, so
// callers that need those decode the raw body into a map instead of this
// struct (see firstEmailValue in users_test.go).
type scimUser struct {
	ID      string   `json:"id"`
	Schemas []string `json:"schemas"`
	Meta    scimMeta `json:"meta"`
}

type scimUserListResponse struct {
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	ItemsPerPage int        `json:"itemsPerPage"`
	Resources    []scimUser `json:"Resources"`
}

type scimGroupMember struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref,omitempty"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type"`
}

type scimGroup struct {
	ID          string            `json:"id"`
	Schemas     []string          `json:"schemas"`
	DisplayName string            `json:"displayName"`
	Members     []scimGroupMember `json:"members"`
	Meta        scimMeta          `json:"meta"`
}

type scimGroupListResponse struct {
	TotalResults int         `json:"totalResults"`
	Resources    []scimGroup `json:"Resources"`
}

type scimPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

type scimPatchRequest struct {
	Schemas    []string      `json:"schemas"`
	Operations []scimPatchOp `json:"Operations"`
}

// scimSearchRequest mirrors backend/internal/scim's SCIMSearchRequest, the
// POST /Users/.search request body (RFC 7644 §3.4.3).
type scimSearchRequest struct {
	Schemas            []string `json:"schemas"`
	Filter             string   `json:"filter,omitempty"`
	Attributes         []string `json:"attributes,omitempty"`
	ExcludedAttributes []string `json:"excludedAttributes,omitempty"`
	StartIndex         int      `json:"startIndex,omitempty"`
	Count              int      `json:"count,omitempty"`
}
