// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
)

var simpleStringTargets = map[string]CoreField{
	"userName":          fieldUserName,
	"displayName":       fieldDisplayName,
	"nickName":          fieldNickName,
	"profileUrl":        fieldProfileURL,
	"title":             fieldTitle,
	"preferredLanguage": fieldPreferredLanguage,
	"locale":            fieldLocale,
	"timezone":          fieldTimezone,
	"password":          fieldPassword,
}

var multiComplexTargets = map[string]struct {
	field    CoreField
	valueKey string
}{
	"emails":              {fieldEmails, "value"},
	"phoneNumbers":        {fieldPhoneNumbers, "value"},
	"photos":              {fieldPhotos, "value"},
	"addresses.formatted": {fieldAddresses, "formatted"},
}

var nameSubAttrs = map[string]struct{}{
	"formatted": {}, "givenName": {}, "familyName": {}, "middleName": {},
}

var addressParts = map[string]struct{}{
	"streetAddress": {}, "locality": {}, "region": {}, "postalCode": {}, "country": {},
}

var enterpriseTargets = map[string]EnterpriseField{
	"employeeNumber": EnterpriseFieldEmployeeNumber,
	"costCenter":     EnterpriseFieldCostCenter,
	"organization":   EnterpriseFieldOrganization,
	"division":       EnterpriseFieldDivision,
	"department":     EnterpriseFieldDepartment,
	"manager":        EnterpriseFieldManager,
}

// BuildRulesFromMapping converts a stored SCIM attribute map into core and enterprise rules, ordered by
// attribute name. meta supplies the type and primary flag of attributes mapped to multi-valued targets.
// Unknown targets are skipped; they are validated against entitytype.ScimTargets when the mapping is saved.
func BuildRulesFromMapping(
	mapping map[string]string, meta map[string]entitytype.ScimAttrMeta,
) ([]CoreAttrRule, []EnterpriseAttrRule) {
	var core []CoreAttrRule
	var enterprise []EnterpriseAttrRule
	candidates := make([]string, 0, len(mapping))
	for candidate := range mapping {
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	for _, candidate := range candidates {
		target := mapping[candidate]
		if field, ok := simpleStringTargets[target]; ok {
			core = append(core, CoreAttrRule{Candidate: candidate, SCIMField: field, Kind: KindSimpleString})
		} else if mc, ok := multiComplexTargets[target]; ok {
			core = append(core, CoreAttrRule{
				Candidate: candidate, SCIMField: mc.field, Kind: KindMultiComplex, ValueKey: mc.valueKey,
				EntryType: meta[candidate].Type, EntryPrimary: meta[candidate].Primary,
			})
		} else if sub, ok := strings.CutPrefix(target, "name."); ok {
			if _, valid := nameSubAttrs[sub]; valid {
				core = append(core, CoreAttrRule{
					Candidate: candidate, Kind: KindSubAttr, ParentField: fieldName, SubAttr: sub,
				})
			}
		} else if sub, ok := strings.CutPrefix(target, "addresses."); ok {
			if _, valid := addressParts[sub]; valid {
				core = append(core, CoreAttrRule{
					Candidate: candidate, Kind: KindMultiComplexPart, ParentField: fieldAddresses, SubAttr: sub,
				})
			}
		} else if field, ok := enterpriseTargets[target]; ok {
			enterprise = append(enterprise, EnterpriseAttrRule{
				Candidate: candidate, SCIMField: field, IsComplex: field == EnterpriseFieldManager,
			})
		}
	}
	return core, enterprise
}
