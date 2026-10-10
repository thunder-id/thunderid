// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"
	"math/rand"
	"strings"
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

// Each change carries its document diffed line by line: all added for a new resource, all removed for
// a deleted one, the changed lines among the kept ones for an update, and nothing when unchanged.
func TestDiffBundlesShowsWhatChangedLineByLine(t *testing.T) {
	held := parseBundle(joinBundle([]string{unit("kept"), unit("changed"), unit("gone")}))
	applied := parseBundle(joinBundle([]string{unit("kept"), unit("changed") + "\ndescription: new", unit("new")}))

	lines := map[string][]LineOp{}
	for _, change := range diffBundles(held, applied) {
		lines[change.ID] = change.Lines
	}

	assert.Nil(t, lines["kept"])
	assert.Equal(t, LineOp{Kind: "+", Text: "description: new"}, lines["changed"][len(lines["changed"])-1])
	for _, op := range lines["changed"][:len(lines["changed"])-1] {
		assert.Equal(t, " ", op.Kind, "a line both versions have was reported as changed: %q", op.Text)
	}
	assert.NotEmpty(t, lines["new"])
	for _, op := range lines["new"] {
		assert.Equal(t, "+", op.Kind)
	}
	assert.NotEmpty(t, lines["gone"])
	for _, op := range lines["gone"] {
		assert.Equal(t, "-", op.Kind)
	}
}

// A replaced line is reported as the old line removed and the new one added, around what is kept.
func TestLineDiffReportsAReplacedLine(t *testing.T) {
	assert.Equal(t, []LineOp{
		{Kind: " ", Text: "name: Orders"},
		{Kind: "-", Text: "description: old"},
		{Kind: "+", Text: "description: new"},
		{Kind: " ", Text: "type: fullstack"},
	}, lineDiff("name: Orders\ndescription: old\ntype: fullstack\n", "name: Orders\ndescription: new\ntype: fullstack"))
	assert.Nil(t, lineDiff("same", "same"))
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

// A line diff keeps as many lines as the longest common subsequence of the two documents, so it
// removes and adds no more than it must, and replaying it gives back both documents.
func TestLineDiffIsAShortestEdit(t *testing.T) {
	random := rand.New(rand.NewSource(1)) //nolint:gosec // A fixed seed keeps the test repeatable.
	document := func() []string {
		lines := make([]string, random.Intn(40))
		for i := range lines {
			lines[i] = string(rune('a' + random.Intn(4)))
		}
		return lines
	}
	for round := 0; round < 5000; round++ {
		a, b := document(), document()
		ops := diffLines(a, b, maxDiffSteps)
		assertReplays(t, a, b, ops)
		assert.Equal(t, commonLines(a, b), countKept(ops), "a: %q\nb: %q", a, b)
	}
}

// Past its budget, a diff reports what is left as removed and added, which still replays.
func TestLineDiffFallsBackToAReplaceOutOfBudget(t *testing.T) {
	random := rand.New(rand.NewSource(2)) //nolint:gosec // A fixed seed keeps the test repeatable.
	for round := 0; round < 500; round++ {
		a := make([]string, random.Intn(30))
		b := make([]string, random.Intn(30))
		for i := range a {
			a[i] = string(rune('a' + random.Intn(3)))
		}
		for i := range b {
			b[i] = string(rune('a' + random.Intn(3)))
		}
		assertReplays(t, a, b, diffLines(a, b, 1+random.Intn(20)))
	}

	ops := diffLines([]string{"keep", "x", "y", "keep"}, []string{"keep", "y", "x", "keep"}, 1)
	assert.Equal(t, []LineOp{
		{Kind: " ", Text: "keep"},
		{Kind: "-", Text: "x"}, {Kind: "-", Text: "y"},
		{Kind: "+", Text: "y"}, {Kind: "+", Text: "x"},
		{Kind: " ", Text: "keep"},
	}, ops)
}

// A large document diffs exactly when a few of its lines change. When every line does, the diff
// stops at its budget and reports the document replaced.
func TestLineDiffOfALargeDocument(t *testing.T) {
	const size = 10000
	from := make([]string, size)
	edited := make([]string, size)
	rewritten := make([]string, size)
	for i := range from {
		from[i] = fmt.Sprintf("line %d", i)
		edited[i] = from[i]
		rewritten[i] = fmt.Sprintf("other %d", i)
	}
	for i := 0; i < size; i += 997 {
		edited[i] = fmt.Sprintf("changed %d", i)
	}

	ops := lineDiff(strings.Join(from, "\n"), strings.Join(edited, "\n"))
	assertReplays(t, from, edited, ops)
	assert.Equal(t, size-11, countKept(ops))

	differ := newLineDiffer(from, rewritten, maxDiffSteps)
	differ.compare(0, size, 0, size)
	assert.LessOrEqual(t, differ.budget, 0, "a document rewritten throughout was diffed within the budget")
	assertReplays(t, from, rewritten, differ.ops)
	assert.Equal(t, 0, countKept(differ.ops))
}

func assertReplays(t *testing.T, a, b []string, ops []LineOp) {
	t.Helper()
	var from, to []string
	for _, op := range ops {
		switch op.Kind {
		case " ":
			from = append(from, op.Text)
			to = append(to, op.Text)
		case "-":
			from = append(from, op.Text)
		case "+":
			to = append(to, op.Text)
		default:
			t.Fatalf("unexpected op %q", op.Kind)
		}
	}
	assert.Equal(t, strings.Join(a, "\n"), strings.Join(from, "\n"))
	assert.Equal(t, strings.Join(b, "\n"), strings.Join(to, "\n"))
}

func countKept(ops []LineOp) int {
	kept := 0
	for _, op := range ops {
		if op.Kind == " " {
			kept++
		}
	}
	return kept
}

// commonLines is the length of the longest common subsequence, by the textbook table.
func commonLines(a, b []string) int {
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table[0][0]
}

// Each reference says which resource holds it and the field it stands in: a list item's field is the
// list's key, and a user is known by its username.
func TestReferencesOfSaysWhatEachValueIsFor(t *testing.T) {
	content := joinBundle([]string{
		"resource_type: application\nid: app-1\nname: Orders\ninboundAuthConfig:\n  - type: oauth2\n" +
			"    config:\n      clientId: var:ORDERS_CLIENT_ID\n      clientSecret: \"sec:ORDERS_SECRET\"\n" +
			"      redirectUris:\n        - var:ORDERS_REDIRECT",
		"resource_type: user\nid: user-1\nattributes:\n  username: alice@example.com\n" +
			"credentials:\n  password: sec:ALICE_PASSWORD",
		"resource_type: translation\nvalue: see var:NOT_A_REFERENCE here",
	})

	assert.Equal(t, []ValueReference{
		{Name: "ORDERS_CLIENT_ID", Kind: ReferenceVariable, ResourceType: "application", ResourceID: "app-1",
			ResourceName: "Orders", Field: "clientId"},
		{Name: "ORDERS_SECRET", Kind: ReferenceSecret, ResourceType: "application", ResourceID: "app-1",
			ResourceName: "Orders", Field: "clientSecret"},
		{Name: "ORDERS_REDIRECT", Kind: ReferenceVariable, ResourceType: "application", ResourceID: "app-1",
			ResourceName: "Orders", Field: "redirectUris", List: true},
		{Name: "ALICE_PASSWORD", Kind: ReferenceSecret, ResourceType: "user", ResourceID: "user-1",
			ResourceName: "alice@example.com", Field: "password"},
	}, referencesOf(content))
}

// Only a user is known by its username: another resource with no name keeps none, even when it
// holds a nested username of its own.
func TestReferencesOfNamesOnlyAUserByItsUsername(t *testing.T) {
	content := "resource_type: connection\nid: conn-1\nproperties:\n  username: var:CONN_USERNAME\n" +
		"  password: sec:CONN_PASSWORD"

	references := referencesOf(content)

	assert.Len(t, references, 2)
	for _, reference := range references {
		assert.Empty(t, reference.ResourceName)
	}
}

// A document naming no resource type is skipped, as an import skips it, so a reference in it is
// neither listed nor counted as needed.
func TestReferencesInADocumentWithoutAResourceTypeAreNotCounted(t *testing.T) {
	content := joinBundle([]string{
		"resource_type: application\nclientId: var:CLIENT_ID",
		"clientId: var:STRAY\nclientSecret: sec:STRAY_SECRET",
	})

	variables, secrets := referencesIn(content)

	assert.Equal(t, []string{"CLIENT_ID"}, variables)
	assert.Empty(t, secrets)
	assert.Len(t, referencesOf(content), 1)
}

// A reference is read under a quoted or dotted key, in block style only: a reference inside flow
// style is not read, since an export never writes one there.
func TestReferencesOfReadsBlockStyleKeysOnly(t *testing.T) {
	content := "resource_type: application\nid: app-1\nname: Orders\n" +
		"\"a.b\": var:QUOTED\n'c d': sec:SINGLE_QUOTED\nsome.key: var:DOTTED\n" +
		"\"list.key\":\n  - var:QUOTED_LIST\n" +
		"flowMap: {a: var:FLOW_MAP}\nflowList: [var:FLOW_LIST]"

	fields := map[string]string{}
	for _, reference := range referencesOf(content) {
		fields[reference.Name] = reference.Field
	}

	assert.Equal(t, map[string]string{
		"QUOTED": "a.b", "SINGLE_QUOTED": "c d", "DOTTED": "some.key", "QUOTED_LIST": "list.key",
	}, fields)
}

// A list item with no key above it has no field.
func TestEnclosingKeyIsEmptyWhenNoKeyEnclosesTheItem(t *testing.T) {
	content := "resource_type: application\n- var:TOP_LEVEL_ITEM"

	references := referencesOf(content)

	assert.Equal(t, []ValueReference{{Name: "TOP_LEVEL_ITEM", Kind: ReferenceVariable,
		ResourceType: "application", List: true}}, references)
	assert.Empty(t, enclosingKey([]string{"- var:X"}, 0, 0))
}
