// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

// scimTargets lists every SCIM target path a stored SCIM mapping can point at.
var scimTargets = []string{
	"userName", "displayName", "nickName", "profileUrl", "title", "preferredLanguage", "locale", "timezone",
	"password",
	"emails", "phoneNumbers", "photos",
	"name.formatted", "name.givenName", "name.familyName", "name.middleName",
	"addresses.formatted", "addresses.streetAddress", "addresses.locality", "addresses.region",
	"addresses.postalCode", "addresses.country",
	"employeeNumber", "costCenter", "organization", "division", "department", "manager",
}

var scimTargetSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(scimTargets))
	for _, target := range scimTargets {
		set[target] = struct{}{}
	}
	return set
}()

// ScimTargets returns every SCIM target path a stored SCIM mapping can point at.
func ScimTargets() []string {
	return append([]string(nil), scimTargets...)
}
