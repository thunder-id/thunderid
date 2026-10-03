// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A version is read document by document, and a document that is not yet YAML, because its
// placeholders are unresolved, is still read for what a diff needs.
func TestParseBundleReadsEachDocument(t *testing.T) {
	content := "# File: a.yaml\nresource_type: application\nid: \"app-1\"\nname: App\nredirectUris:\n" +
		"{{- range .URIS}}\n  - {{.}}\n{{- end}}\n---\nnot a resource\n---\nresource_type: translation\n" +
		"name: en-US\n"

	resources := parseBundle(content)

	assert.Len(t, resources, 2, "a document naming no resource type is skipped")
	assert.Equal(t, "application", resources[0].Type)
	assert.Equal(t, "app-1", resources[0].ID, "quotes are taken off")
	assert.Equal(t, "application/app-1", resources[0].key())
	assert.Equal(t, "translation/en-US", resources[1].key(), "a resource without an id is keyed by name")
}

// The diff names each resource by how it differs, deletions included.
func TestDiffBundlesClassifiesEveryChange(t *testing.T) {
	held := parseBundle(joinBundle([]string{unit("kept"), unit("changed"), unit("gone")}))
	applied := parseBundle(joinBundle([]string{unit("kept"), unit("changed") + "\ndescription: new", unit("new")}))

	changes := diffBundles(held, applied)

	kinds := map[string]ChangeKind{}
	for _, change := range changes {
		kinds[change.ID] = change.Change
	}
	assert.Equal(t, map[string]ChangeKind{
		"kept": ChangeUnchanged, "changed": ChangeUpdated, "gone": ChangeDeleted, "new": ChangeAdded,
	}, kinds)
	assert.Equal(t, DiffSummary{Added: 1, Updated: 1, Deleted: 1, Unchanged: 1}, summarize(changes))
}

// Only a removed resource with an id can be named to the gateway for deletion.
func TestDeletionsNeedAnID(t *testing.T) {
	deletions := deletionsOf([]Change{
		{ResourceType: "organization_unit", ID: "ou-1", Change: ChangeDeleted},
		{ResourceType: "translation", Name: "en-US", Change: ChangeDeleted},
		{ResourceType: "organization_unit", ID: "ou-2", Change: ChangeAdded},
	})

	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"}}, deletions)
}

func TestParseEnvFileReadsKeyValueLines(t *testing.T) {
	assert.Equal(t, map[string]string{"A": "1", "B": "x=y"},
		parseEnvFile("A=1\n\n# note\nB=x=y\nnot-a-pair\n"))
}

// A list is given to an import as a list; anything else, including text that only looks like one,
// stays a string.
func TestImportVariablesGivesListsAsLists(t *testing.T) {
	assert.Equal(t, map[string]interface{}{
		"LIST": []interface{}{"a", "b"}, "TEXT": "plain", "BROKEN": "[not json",
	}, importVariables(map[string]string{"LIST": `["a","b"]`, "TEXT": "plain", "BROKEN": "[not json"}))
}

// A reference counts only where it stands as a whole value, and each is named once.
func TestReferencesInFindsWholeValueReferences(t *testing.T) {
	content := joinBundle([]string{
		"resource_type: application\nclient_id: var:CLIENT_ID\nclient_secret: 'sec:CLIENT_SECRET'\n" +
			"redirect_uris:\n  - \"var:REDIRECT\"\n  - var:CLIENT_ID",
		"resource_type: translation\nvalue: see var:NOT_A_REFERENCE here\nnote: \"sec:\"",
	})

	variables, secrets := referencesIn(content)

	assert.Equal(t, []string{"CLIENT_ID", "REDIRECT"}, variables)
	assert.Equal(t, []string{"CLIENT_SECRET"}, secrets)
}
