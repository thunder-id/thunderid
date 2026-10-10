// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/entitytype"
)

func TestBuildRulesFromMapping_Core(t *testing.T) {
	tests := []struct {
		name     string
		mapping  map[string]string
		expected []CoreAttrRule
	}{
		{"simple string", map[string]string{"email": "userName"},
			[]CoreAttrRule{{Candidate: "email", SCIMField: fieldUserName, Kind: KindSimpleString}}},
		{"password", map[string]string{"pwd": "password"},
			[]CoreAttrRule{{Candidate: "pwd", SCIMField: fieldPassword, Kind: KindSimpleString}}},
		{"name sub attribute", map[string]string{"given_name": "name.givenName"},
			[]CoreAttrRule{{Candidate: "given_name", Kind: KindSubAttr, ParentField: fieldName, SubAttr: "givenName"}}},
		{"multi complex", map[string]string{"work_email": "emails"},
			[]CoreAttrRule{{
				Candidate: "work_email", SCIMField: fieldEmails, Kind: KindMultiComplex, ValueKey: "value",
			}}},
		{"formatted address is multi complex", map[string]string{"addr": "addresses.formatted"},
			[]CoreAttrRule{{
				Candidate: "addr", SCIMField: fieldAddresses, Kind: KindMultiComplex, ValueKey: "formatted",
			}}},
		{"address part", map[string]string{"street": "addresses.streetAddress"},
			[]CoreAttrRule{{Candidate: "street", Kind: KindMultiComplexPart, ParentField: fieldAddresses,
				SubAttr: "streetAddress"}}},
		{"unknown target skipped", map[string]string{"x": "notATarget", "y": "name.nope", "z": "addresses.nope"}, nil},
		{"empty", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, enterprise := BuildRulesFromMapping(tt.mapping, nil)
			assert.Equal(t, tt.expected, core)
			assert.Empty(t, enterprise)
		})
	}
}

func TestBuildRulesFromMapping_MultipleAttributesOneMultiValuedTarget(t *testing.T) {
	core, _ := BuildRulesFromMapping(map[string]string{"work_email": "emails", "home_email": "emails"}, nil)
	assert.ElementsMatch(t, []CoreAttrRule{
		{Candidate: "work_email", SCIMField: fieldEmails, Kind: KindMultiComplex, ValueKey: "value"},
		{Candidate: "home_email", SCIMField: fieldEmails, Kind: KindMultiComplex, ValueKey: "value"},
	}, core)
}

func TestBuildRulesFromMapping_MultiValuedMetaAndOrder(t *testing.T) {
	core, _ := BuildRulesFromMapping(
		map[string]string{"work_email": "emails", "home_email": "emails"},
		map[string]entitytype.ScimAttrMeta{"work_email": {Type: "work", Primary: true}},
	)
	assert.Equal(t, []CoreAttrRule{
		{Candidate: "home_email", SCIMField: fieldEmails, Kind: KindMultiComplex, ValueKey: "value"},
		{Candidate: "work_email", SCIMField: fieldEmails, Kind: KindMultiComplex, ValueKey: "value",
			EntryType: "work", EntryPrimary: true},
	}, core)
}

func TestBuildRulesFromMapping_Enterprise(t *testing.T) {
	core, enterprise := BuildRulesFromMapping(map[string]string{"emp": "employeeNumber", "mgr": "manager"}, nil)
	assert.Empty(t, core)
	assert.ElementsMatch(t, []EnterpriseAttrRule{
		{Candidate: "emp", SCIMField: EnterpriseFieldEmployeeNumber},
		{Candidate: "mgr", SCIMField: EnterpriseFieldManager, IsComplex: true},
	}, enterprise)
}

func TestBuildRulesFromMapping_HandlesEveryEntityTypeScimTarget(t *testing.T) {
	for _, target := range entitytype.ScimTargets() {
		core, enterprise := BuildRulesFromMapping(map[string]string{"attr": target}, nil)
		assert.Equal(t, 1, len(core)+len(enterprise), "target %q produced no rule", target)
	}
}
