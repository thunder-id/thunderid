// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package secretresolver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/valueref"
)

// heldValues is a store holding each value under its collection and name.
func heldValues(values map[valueref.Collection]map[string]string) Lookup {
	return func(_ context.Context, collection valueref.Collection, name string) (string, bool, error) {
		value, held := values[collection][name]
		return value, held, nil
	}
}

var store = heldValues(map[valueref.Collection]map[string]string{
	valueref.CollectionVariable: {"CLIENT_ID": "console", "PORT": "8090"},
	valueref.CollectionSecret:   {"CLIENT_SECRET": "s3cr\"et: with\nnewline"},
})

// A reference is replaced with what the store holds in the collection its prefix names, and anything
// else passes through unchanged.
func TestResolveReplacesAReferenceWithItsValue(t *testing.T) {
	r := New(store)

	for value, want := range map[string]string{
		"var:CLIENT_ID":     "console",
		"sec:CLIENT_SECRET": "s3cr\"et: with\nnewline",
		"plain value":       "plain value",
		"":                  "",
	} {
		got, err := r.Resolve(context.Background(), value)
		require.NoError(t, err, value)
		assert.Equal(t, want, got, value)
	}
}

// The prefix decides the collection: a name held only as a variable is not found as a secret.
func TestResolveReadsTheCollectionThePrefixNames(t *testing.T) {
	_, err := New(store).Resolve(context.Background(), "sec:CLIENT_ID")

	assert.ErrorIs(t, err, ErrNotHeld)
}

// A store that cannot be read is reported as such, not as a missing value.
func TestResolveReportsAStoreFailure(t *testing.T) {
	failing := func(context.Context, valueref.Collection, string) (string, bool, error) {
		return "", false, errors.New("database is down")
	}

	_, err := New(failing).Resolve(context.Background(), "var:CLIENT_ID")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotHeld)
}

func parse(t *testing.T, document string) *yaml.Node {
	t.Helper()
	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(document), &node))
	return &node
}

func render(t *testing.T, node *yaml.Node) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, node.Decode(&decoded))
	return decoded
}

// Every whole-value reference in a document is replaced, in mappings and lists alike, and a value is
// kept as a string even where it reads as a number or holds YAML punctuation.
func TestResolveNodeReplacesEveryReferenceInADocument(t *testing.T) {
	node := parse(t, `
name: app
clientId: var:CLIENT_ID
clientSecret: "sec:CLIENT_SECRET"
port: var:PORT
note: see var:CLIENT_ID here
var:CLIENT_ID: a key is not a value
redirectUris:
  - var:CLIENT_ID
`)

	require.NoError(t, New(store).ResolveNode(context.Background(), node))

	assert.Equal(t, map[string]any{
		"name":          "app",
		"clientId":      "console",
		"clientSecret":  "s3cr\"et: with\nnewline",
		"port":          "8090",
		"note":          "see var:CLIENT_ID here",
		"var:CLIENT_ID": "a key is not a value",
		"redirectUris":  []any{"console"},
	}, render(t, node))
}

// A document with a reference the store does not hold is left as it was, and the error names every
// such reference once.
func TestResolveNodeNamesWhatIsNotHeld(t *testing.T) {
	node := parse(t, `
clientId: var:CLIENT_ID
clientSecret: sec:MISSING_SECRET
other: var:MISSING_VAR
again: sec:MISSING_SECRET
`)

	err := New(store).ResolveNode(context.Background(), node)

	var unresolved *UnresolvedError
	require.ErrorAs(t, err, &unresolved)
	assert.ErrorIs(t, err, ErrNotHeld)
	assert.Equal(t, []string{"sec:MISSING_SECRET", "var:MISSING_VAR"}, unresolved.References)
	assert.Contains(t, err.Error(), "sec:MISSING_SECRET")
	assert.Equal(t, "var:CLIENT_ID", render(t, node)["clientId"], "a document was changed though it was refused")
}

// A store failure stops resolution with that failure rather than a list of missing names.
func TestResolveNodeReportsAStoreFailure(t *testing.T) {
	failing := func(context.Context, valueref.Collection, string) (string, bool, error) {
		return "", false, errors.New("database is down")
	}

	err := New(failing).ResolveNode(context.Background(), parse(t, "clientId: var:CLIENT_ID"))

	require.Error(t, err)
	var unresolved *UnresolvedError
	assert.False(t, errors.As(err, &unresolved))
}

// A document without references, or with none at all, is left untouched.
func TestResolveNodeLeavesADocumentWithoutReferences(t *testing.T) {
	node := parse(t, "name: app\nlist: [a, b]")

	require.NoError(t, New(store).ResolveNode(context.Background(), node))
	require.NoError(t, New(store).ResolveNode(context.Background(), nil))

	assert.Equal(t, map[string]any{"name": "app", "list": []any{"a", "b"}}, render(t, node))
}
