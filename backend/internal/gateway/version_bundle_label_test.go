// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Each change is named as a person knows the resource, whatever its type: a name, a display name or a
// handle, or a user's username, email or name; a resource with none of them is left unnamed.
func TestEachChangeIsNamedFromItsDocument(t *testing.T) {
	for _, c := range []struct {
		document string
		want     string
	}{
		{"resource_type: theme\nid: th-1\nhandle: shared-teal\ndisplayName: Shared Teal\ntheme: {}", "Shared Teal"},
		{"resource_type: layout\nid: ly-1\nhandle: centered\ndisplayName: Centered\nlayout: {}", "Centered"},
		{"resource_type: layout\nid: ly-2\nhandle: wide\nlayout: {}", "wide"},
		{"resource_type: user_type\nid: ut-1\ncategory: user\nhandle: person\ndisplayName: Person\nschema: {}",
			"Person"},
		{"resource_type: organization_unit\nid: ou-1\nhandle: default\nname: Default", "Default"},
		{"resource_type: organization_unit\nid: ou-2\nhandle: team", "team"},
		{"resource_type: user\nid: u-1\nattributes:\n  email: \"sam@example.com\"\n  name: \"Sam\"\n" +
			"  username: \"sam\"", "sam"},
		{"resource_type: user\nid: u-2\nattributes:\n  email: \"kim@example.com\"\n  name: \"Kim\"", "kim@example.com"},
		{"resource_type: user\nid: u-3\nattributes:\n  name: \"Lee\"", "Lee"},
		{"resource_type: group\nid: g-1\nname: Admins\nmembers:\n  - id: u-1", "Admins"},
		{"resource_type: agent\nid: ag-1\ntype: default\nowner: u-1\nattributes:\n  description: x\n" +
			"  name: \"Billing Bot\"\ninboundAuthConfig:\n  - type: oauth2\n    name: client", "Billing Bot"},
		{"resource_type: agent\nid: ag-2\ntype: default\ninboundAuthConfig:\n  - name: client", ""},
		{"resource_type: user\nid: u-4\nattributes:\n  address:\n    name: \"Home\"\n  email: \"a@b.c\"",
			"a@b.c"},
		{"resource_type: role\nid: r-1\npermissions:\n  - name: nested", ""},
		{"resource_type: translation\nlanguage: en-US\ntext: hi", "en-US"},
	} {
		resources := parseBundle(c.document)
		changes := diffBundles(nil, resources)
		if assert.Len(t, changes, 1, c.document) {
			assert.Equal(t, c.want, changes[0].Name, c.document)
			assert.Equal(t, resources[0].key(), changes[0].Key, "a label changed the key of %s", c.document)
		}
	}
}

// An agent needs the user that owns it, when the version holds that user, so the user goes with it.
func TestAnAgentNeedsItsOwner(t *testing.T) {
	needs := needsOf(parseBundle(joinBundle([]string{
		"resource_type: user\nid: u-1\nattributes:\n  username: sam",
		"resource_type: agent\nid: ag-1\ntype: default\nowner: u-1\nattributes:\n  name: bot",
	})))

	assert.Equal(t, []string{"user/u-1"}, needs["agent/ag-1"])
}
