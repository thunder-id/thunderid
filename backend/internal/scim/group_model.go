package scim

import "encoding/json"

// SCIMGroupMember represents a member in a SCIM Group resource (RFC 7643 §4.2).
type SCIMGroupMember struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref"`
	Display string `json:"display"`
	Type    string `json:"type"`
}

// SCIMGroup is the SCIM wire representation of a ThunderID group resource.
type SCIMGroup struct {
	ID          string            `json:"id"`
	Schemas     []string          `json:"schemas"`
	DisplayName string            `json:"displayName"`
	Members     []SCIMGroupMember `json:"members"`
	Meta        SCIMMeta          `json:"meta"`
}

// SCIMGroupListResponse is the SCIM ListResponse envelope for Group resources (RFC 7644 §3.4.2).
type SCIMGroupListResponse struct {
	Schemas      []string    `json:"schemas"`
	TotalResults int         `json:"totalResults"`
	StartIndex   int         `json:"startIndex"`
	ItemsPerPage int         `json:"itemsPerPage"`
	Resources    []SCIMGroup `json:"Resources"`
}

// SCIMGroupPatchOp is a single operation in a PATCH request (RFC 7644 §3.5.2).
type SCIMGroupPatchOp struct {
	Op    string          `json:"op"` // "add", "remove", "replace"
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// SCIMGroupPatchRequest is the top-level PATCH body.
type SCIMGroupPatchRequest struct {
	Schemas    []string           `json:"schemas"`
	Operations []SCIMGroupPatchOp `json:"Operations"`
}
