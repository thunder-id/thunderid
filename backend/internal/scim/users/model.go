// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"encoding/json"
	"fmt"

	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

// SCIMUser is the SCIM wire representation of a ThunderID user resource.
type SCIMUser struct {
	ID              string                     `json:"id"`
	Schemas         []string                   `json:"schemas"`
	ExtensionURN    string                     `json:"-"`
	Attributes      json.RawMessage            `json:"-"`
	CoreAttrs       map[string]json.RawMessage `json:"-"`
	EnterpriseAttrs json.RawMessage            `json:"-"`
	Meta            scim.SCIMMeta              `json:"meta"`
}

// MarshalJSON produces the SCIM wire JSON for a User resource.
func (u SCIMUser) MarshalJSON() ([]byte, error) {
	type plain struct {
		Schemas []string      `json:"schemas"`
		ID      string        `json:"id"`
		Meta    scim.SCIMMeta `json:"meta"`
	}
	base := plain{
		Schemas: u.Schemas,
		ID:      u.ID,
		Meta:    u.Meta,
	}

	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("SCIMUser.MarshalJSON: failed to marshal base: %w", err)
	}

	hasExtension := len(u.Attributes) > 0 && u.ExtensionURN != ""
	hasEnterprise := len(u.EnterpriseAttrs) > 0
	if !hasExtension && !hasEnterprise && len(u.CoreAttrs) == 0 {
		return baseBytes, nil
	}

	// Merge extension attributes under the URN key into the base map.
	var baseMap map[string]json.RawMessage
	if err := json.Unmarshal(baseBytes, &baseMap); err != nil {
		return nil, fmt.Errorf("SCIMUser.MarshalJSON: failed to unmarshal base map: %w", err)
	}
	if len(u.Attributes) > 0 && u.ExtensionURN != "" {
		baseMap[u.ExtensionURN] = u.Attributes
	}
	if len(u.EnterpriseAttrs) > 0 {
		baseMap[scim.SCIMEnterpriseUserSchemaURN] = u.EnterpriseAttrs
	}
	// Merge core attributes directly into the top level
	for k, v := range u.CoreAttrs {
		baseMap[k] = v
	}

	return json.Marshal(baseMap)
}

// SCIMUserListResponse is the SCIM ListResponse envelope for User resources.
type SCIMUserListResponse struct {
	Schemas      []string   `json:"schemas"`
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	ItemsPerPage int        `json:"itemsPerPage"`
	Resources    []SCIMUser `json:"Resources"`
}
