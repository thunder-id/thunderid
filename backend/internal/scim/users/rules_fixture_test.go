// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
)

var testScimAttributeMap = map[string]string{
	"username":           "userName",
	"email":              "emails",
	"given_name":         "name.givenName",
	"family_name":        "name.familyName",
	"middle_name":        "name.middleName",
	"name":               "name.formatted",
	"phone_number":       "phoneNumbers",
	"display_name":       "displayName",
	"nickname":           "nickName",
	"picture":            "photos",
	"locale":             "locale",
	"preferred_language": "preferredLanguage",
	"zoneinfo":           "timezone",
	"profile":            "profileUrl",
	"title":              "title",
	"address":            "addresses.formatted",
	"street_address":     "addresses.streetAddress",
	"locality":           "addresses.locality",
	"region":             "addresses.region",
	"postal_code":        "addresses.postalCode",
	"country":            "addresses.country",
	"employee_number":    "employeeNumber",
	"cost_center":        "costCenter",
	"organization":       "organization",
	"division":           "division",
	"department":         "department",
	"manager":            "manager",
}

var testCoreRules, testEnterpriseRules = scim.BuildRulesFromMapping(testScimAttributeMap, nil)

var testScimCoreAttrs = &entitytype.SystemAttributes{
	IsScimCoreType: true,
	ScimMapping:    &entitytype.ScimMapping{AttributeMap: testScimAttributeMap},
}

func translateSCIMFilterAttr(attr string) string {
	for key := range translateSCIMFilters(map[string]interface{}{attr: true}, testCoreRules, testEnterpriseRules) {
		return key
	}
	return attr
}
