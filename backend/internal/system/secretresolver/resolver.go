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
	// secretFields names, by resource type, the fields a document may refer to a secret from. A
	// resource type with no entry may refer to none.
	secretFields map[string][]string
}

// New builds a Resolver that reads values through lookup. Until WithSecretFields names where a
// secret may stand, no document may refer to one.
func New(lookup Lookup) *Resolver {
	return &Resolver{lookup: lookup}
}

// WithSecretFields names, by resource type, the fields a document may refer to a secret from, as
// declarativeresource.SecretFieldsOf collects them, and returns the resolver for chaining.
func (r *Resolver) WithSecretFields(fields map[string][]string) *Resolver {
	r.secretFields = fields
	return r
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

// MisplacedSecretError names every field a document refers to a secret from that is not one a
// credential is kept in.
type MisplacedSecretError struct {
	ResourceType string
	Fields       []string
}

func (e *MisplacedSecretError) Error() string {
	return "a " + e.ResourceType + " may not refer to a secret from " + strings.Join(e.Fields, ", ")
}

// ResolveDocument replaces, in place, every reference in a parsed document of the given resource
// type with the value it names.
//
// It works on the parsed nodes rather than on the text, so a value containing a quote or a newline
// cannot break the document it is put into. Only a scalar that is wholly a reference is one: the same
// text inside a longer value is left as it is. A resolved value is tagged as a string, so one that
// reads as a number or a boolean is not reinterpreted.
//
// A secret is resolved only where the resource keeps a credential, the fields WithSecretFields names
// for the resource type. Anywhere else the value would be stored as an ordinary one and could be read
// back, by an API or by whatever the field is sent to, by whoever wrote the document. Such a document
// is refused with a MisplacedSecretError before the store is read. A secret reference under a YAML
// anchor is refused too, since an alias would put its resolved value wherever the alias stands.
//
// When the store lacks a value, the document is left unresolved and an UnresolvedError names every
// reference it lacks, so the resource is refused rather than written with the reference text as its
// value.
func (r *Resolver) ResolveDocument(ctx context.Context, resourceType string, node *yaml.Node) error {
	var references []*yaml.Node
	var misplaced []MisplacedSecret
	collectReferences(node, "", false, r.secretFields[resourceType], &references, &misplaced)
	if len(misplaced) > 0 {
		fields := make([]string, 0, len(misplaced))
		for _, secret := range misplaced {
			fields = append(fields, secret.Field)
		}
		return &MisplacedSecretError{ResourceType: resourceType, Fields: fields}
	}

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

// MisplacedSecret is a secret reference standing in a field where no credential is kept.
type MisplacedSecret struct {
	// Field is where the reference stands, as SecretFieldLister names fields.
	Field string
	// Reference is the reference as written, such as "sec:NAME".
	Reference string
}

// MisplacedSecrets finds, without resolving anything, every secret reference in a parsed document
// that stands outside the fields a credential is kept in, or under a YAML anchor. It is what
// ResolveDocument refuses a document for, offered on its own so a document can be checked where no
// value store is at hand, as on a control plane authoring it.
func MisplacedSecrets(node *yaml.Node, secretFields []string) []MisplacedSecret {
	var references []*yaml.Node
	var misplaced []MisplacedSecret
	collectReferences(node, "", false, secretFields, &references, &misplaced)
	return misplaced
}

// collectReferences gathers every scalar node that holds a reference, and separately every secret
// reference that stands where no credential is kept, or under an anchor.
//
// path is the node's field as SecretFieldLister names it: keys joined by dots, "[]" for a list item.
func collectReferences(node *yaml.Node, path string, anchored bool, secretFields []string,
	into *[]*yaml.Node, misplaced *[]MisplacedSecret) {
	if node == nil {
		return
	}
	anchored = anchored || node.Anchor != ""
	if node.Kind == yaml.ScalarNode {
		if !valueref.IsWellFormedReference(node.Value) {
			return
		}
		if valueref.IsSecretReference(node.Value) && (anchored || !fieldAllowed(path, secretFields)) {
			*misplaced = append(*misplaced, MisplacedSecret{Field: path, Reference: node.Value})
			return
		}
		*into = append(*into, node)
		return
	}
	for i, child := range node.Content {
		// A mapping's keys name fields; only its values can hold a reference.
		if node.Kind == yaml.MappingNode && i%2 == 0 {
			continue
		}
		collectReferences(child, childPath(node, path, i), anchored, secretFields, into, misplaced)
	}
}

// childPath names the field a node's child stands in.
//
// A key is escaped so that it can only name one field: a single key written "credentials.password"
// or "inboundAuthConfig[].config.clientSecret" would otherwise read as the nested field of that
// name and pass as one.
func childPath(parent *yaml.Node, path string, index int) string {
	switch parent.Kind {
	case yaml.MappingNode:
		key := keyEscaper.Replace(parent.Content[index-1].Value)
		if path == "" {
			return key
		}
		return path + "." + key
	case yaml.SequenceNode:
		return path + "[]"
	default:
		return path
	}
}

// keyEscaper keeps a key's own backslashes, dots and brackets from reading as the path's separators.
var keyEscaper = strings.NewReplacer(`\`, `\\`, ".", `\.`, "[", `\[`, "]", `\]`)

// hasUnescapedDot reports whether a path goes down another level: a dot that is a separator, not
// one escaped as part of a key.
func hasUnescapedDot(path string) bool {
	escaped := false
	for _, r := range path {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '.':
			return true
		}
	}
	return false
}

// fieldAllowed reports whether a secret may stand in the field: it is named exactly, or its
// mapping is named with a trailing ".*", which covers the mapping's own keys, dotted ones included,
// and nothing beneath them. A field named with a leading "!" is excluded, even from a ".*" mapping.
func fieldAllowed(path string, secretFields []string) bool {
	for _, field := range secretFields {
		if excluded, ok := strings.CutPrefix(field, "!"); ok && excluded == path {
			return false
		}
	}
	for _, field := range secretFields {
		if field == path {
			return true
		}
		if prefix, ok := strings.CutSuffix(field, ".*"); ok && strings.HasPrefix(path, prefix+".") &&
			!hasUnescapedDot(strings.TrimPrefix(path, prefix+".")) {
			return true
		}
	}
	return false
}
