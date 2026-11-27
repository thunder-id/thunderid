// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federatedsync

import (
	"encoding/json"
	"reflect"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// resolvedMappedAttributes returns the local attributes whose connection mapping finds its external
// attribute in claims.
func resolvedMappedAttributes(claims map[string]interface{},
	mappings []providers.AttributeMapping) map[string]bool {
	mapped := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		if _, ok := systemutils.GetNestedValue(claims, mapping.ExternalAttribute); ok {
			mapped[mapping.LocalAttribute] = true
		}
	}
	return mapped
}

// jsonEqual reports whether a and b hold the same value once encoded, so a stored number and a received
// one of a different Go type compare equal. Values that can't be encoded are never equal.
func jsonEqual(a, b interface{}) bool {
	normalize := func(v interface{}) (interface{}, bool) {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var out interface{}
		if err := json.Unmarshal(encoded, &out); err != nil {
			return nil, false
		}
		return out, true
	}
	normalizedA, okA := normalize(a)
	normalizedB, okB := normalize(b)
	return okA && okB && reflect.DeepEqual(normalizedA, normalizedB)
}

// groupMemberType returns how an entity of the category is named as a group member.
func groupMemberType(category providers.EntityCategory) group.MemberType {
	switch category {
	case providers.EntityCategoryApp:
		return group.MemberTypeApp
	case providers.EntityCategoryAgent:
		return group.MemberTypeAgent
	default:
		return group.MemberTypeUser
	}
}

// roleAssigneeType returns how an entity of the category is named as a role assignee.
func roleAssigneeType(category providers.EntityCategory) role.AssigneeType {
	switch category {
	case providers.EntityCategoryApp:
		return role.AssigneeTypeApp
	case providers.EntityCategoryAgent:
		return role.AssigneeTypeAgent
	default:
		return role.AssigneeTypeUser
	}
}
