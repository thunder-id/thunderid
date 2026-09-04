// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"encoding/json"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// SCIMGroupMember represents a member in a SCIM Group resource (RFC 7643 §4.2).
// Display is server-computed only: it is always populated from the referenced
// resource's own display attribute on read and any value a client sends in it on
// write is ignored, never stored, and never echoed back.
type SCIMGroupMember struct {
	Value   string `json:"value"`
	Ref     string `json:"$ref,omitempty"`
	Display string `json:"display,omitempty"`
	Type    string `json:"type,omitempty"`
}

// SCIMGroup is the SCIM wire representation of a ThunderID group resource.
type SCIMGroup struct {
	ID          string            `json:"id"`
	Schemas     []string          `json:"schemas"`
	DisplayName string            `json:"displayName"`
	Members     []SCIMGroupMember `json:"members"`
	Meta        scim.SCIMMeta     `json:"meta"`
}

// SCIMGroupListResponse is the SCIM ListResponse envelope for Group resources (RFC 7644 §3.4.2).
type SCIMGroupListResponse struct {
	Schemas      []string    `json:"schemas"`
	TotalResults int         `json:"totalResults"`
	StartIndex   int         `json:"startIndex"`
	ItemsPerPage int         `json:"itemsPerPage"`
	Resources    []SCIMGroup `json:"Resources"`
}

// scimGroupPatchOp is a single operation in a PATCH request (RFC 7644 §3.5.2).
type scimGroupPatchOp struct {
	Op    string          `json:"op"` // "add", "remove", "replace"
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// scimGroupPatchRequest is the top-level PATCH body.
type scimGroupPatchRequest struct {
	Schemas    []string           `json:"schemas"`
	Operations []scimGroupPatchOp `json:"Operations"`
}

// scimGroupPayload is the parsed, validated result of a SCIM Group POST/PUT request body.
type scimGroupPayload struct {
	DisplayName string
	Members     []SCIMGroupMember
}

// scimGroupPatchTarget identifies which attribute a validated PATCH action applies to.
const (
	scimGroupPatchTargetDisplayName = "displayName"
	scimGroupPatchTargetMembers     = "members"
)

// SCIMGroupPatchAction is a single normalized, validated PATCH operation ready to apply
// to a group (RFC 7644 §3.5.2).
type SCIMGroupPatchAction struct {
	// Op is one of scimPatchOpAdd, scimPatchOpRemove, scimPatchOpReplace.
	Op string
	// Target is one of scimGroupPatchTargetDisplayName, scimGroupPatchTargetMembers.
	Target string
	// DisplayName holds the new value when Target is scimGroupPatchTargetDisplayName.
	DisplayName string
	// FilterValue holds the member id extracted from a members[value eq "<id>"] path,
	// used for a filtered remove. Empty for an unfiltered members op.
	FilterValue string
	// Members holds the member list for add/replace ops targeting members.
	Members []SCIMGroupMember
}
