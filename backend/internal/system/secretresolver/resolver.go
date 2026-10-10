// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package secretresolver replaces a reference such as "sec:MY_APP_CLIENT_SECRET" or "var:DB_HOST" with
// the value this deployment's variable store holds under that name.
//
// Configuration from a control plane carries references in place of values, and the values live on the
// deployment that uses them. Resolving puts the value where the reference stood before the resource is
// written, so the service writing it stores the value as it stores any other: a credential hashed, a
// secret property encrypted, an identifier as it is.
package secretresolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/valueref"
)

// ErrNotHeld is returned when the store holds no value for a reference.
var ErrNotHeld = errors.New("no value is held for the reference")

// Lookup reads the value held under a name in one of the store's collections, and whether one is.
type Lookup func(ctx context.Context, collection valueref.Collection, name string) (string, bool, error)

// Resolver replaces references with the values a Lookup returns.
type Resolver struct {
	lookup Lookup
}

// New builds a Resolver that reads values through lookup.
func New(lookup Lookup) *Resolver {
	return &Resolver{lookup: lookup}
}

// Resolve returns value with a reference replaced by the value it names. A value that is not a
// reference is returned unchanged, so any configuration value can be passed through.
func (r *Resolver) Resolve(ctx context.Context, value string) (string, error) {
	collection, name, isRef := valueref.ParseReference(value)
	if !isRef {
		return value, nil
	}
	held, found, err := r.lookup(ctx, collection, name)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", value, err)
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrNotHeld, value)
	}
	return held, nil
}

// UnresolvedError names every reference in a document that the store holds no value for.
type UnresolvedError struct {
	References []string
}

func (e *UnresolvedError) Error() string {
	return "no value is held for " + strings.Join(e.References, ", ")
}

// Is lets a caller match an UnresolvedError against ErrNotHeld.
func (e *UnresolvedError) Is(target error) bool {
	return target == ErrNotHeld
}

// ResolveNode replaces, in place, every reference in a parsed document with the value it names.
//
// It works on the parsed nodes rather than on the text, so a value containing a quote or a newline
// cannot break the document it is put into. Only a scalar that is wholly a reference is one: the same
// text inside a longer value is left as it is. A resolved value is tagged as a string, so one that
// reads as a number or a boolean is not reinterpreted.
//
// When the store lacks a value, the document is left unresolved and an UnresolvedError names every
// reference it lacks, so the resource is refused rather than written with the reference text as its
// value.
func (r *Resolver) ResolveNode(ctx context.Context, node *yaml.Node) error {
	var references []*yaml.Node
	collectReferences(node, &references)

	values := make(map[*yaml.Node]string, len(references))
	variables := make(map[*yaml.Node]string, len(references))
	missing := map[string]bool{}
	for _, reference := range references {
		value, err := r.Resolve(ctx, reference.Value)
		if errors.Is(err, ErrNotHeld) {
			missing[reference.Value] = true
			continue
		}
		if err != nil {
			return err
		}
		values[reference] = value
		if !valueref.IsSecretReference(reference.Value) {
			variables[reference] = value
		}
	}
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		return &UnresolvedError{References: names}
	}

	for reference, value := range values {
		reference.Value = value
		reference.Tag = "!!str"
		reference.Style = 0
	}
	expandLists(node, variables)
	return nil
}

// expandLists replaces each list item resolved from a variable whose value is a JSON list of strings
// with one item per string. A list in a document is exported as a list of one reference, and the
// variable it names holds however many items this deployment uses. A value that is not such a list
// stays a single item.
//
// Only variables expand: an export refers to a list only through a variable, since what it writes as
// a list is an address list, never a credential, so a secret is always one value. Every list in every
// resolved document is looked at, not only redirect URIs, because which lists an export refers to is
// the export's to say. An alias to a resolved item expands as the item does.
func expandLists(node *yaml.Node, resolved map[*yaml.Node]string) {
	if node == nil {
		return
	}
	if node.Kind == yaml.SequenceNode {
		items := make([]*yaml.Node, 0, len(node.Content))
		for _, item := range node.Content {
			target := item
			if item.Kind == yaml.AliasNode && item.Alias != nil {
				target = item.Alias
			}
			value, wasReference := resolved[target]
			listed, isList := listOf(value)
			if !wasReference || !isList {
				items = append(items, item)
				continue
			}
			for _, entry := range listed {
				items = append(items, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: entry})
			}
		}
		node.Content = items
	}
	for _, child := range node.Content {
		expandLists(child, resolved)
	}
}

// listOf reads a value written as a JSON list of strings. A list holding anything but strings, a null
// included, is not one, so it is not expanded into items it does not name.
func listOf(value string) ([]string, bool) {
	if !strings.HasPrefix(strings.TrimSpace(value), "[") {
		return nil, false
	}
	var entries []any
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil, false
	}
	listed := make([]string, 0, len(entries))
	for _, entry := range entries {
		text, ok := entry.(string)
		if !ok {
			return nil, false
		}
		listed = append(listed, text)
	}
	return listed, true
}

// collectReferences gathers every scalar node that holds a reference.
func collectReferences(node *yaml.Node, into *[]*yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode {
		if valueref.IsWellFormedReference(node.Value) {
			*into = append(*into, node)
		}
		return
	}
	for i, child := range node.Content {
		// A mapping's keys name fields; only its values can hold a reference.
		if node.Kind == yaml.MappingNode && i%2 == 0 {
			continue
		}
		collectReferences(child, into)
	}
}
