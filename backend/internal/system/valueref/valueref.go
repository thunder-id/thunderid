// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package valueref is the syntax a stored configuration value uses to name a value it does not
// carry.
//
// A control plane authors configuration but does not hold the values that configuration refers to:
// those belong to the data plane that runs them. What it stores in their place is a reference, and
// this is the shape of one.
//
// The prefix names the collection the value is held in, and that is what tells a credential apart
// from an ordinary value. A data plane returns a variable's value to a read and never a secret's, so
// a reference that did not say which it meant could put a credential where a read returns it.
package valueref

import "strings"

// The prefixes a reference carries.
const (
	// PrefixSecret refers to a credential, whose value a read never returns.
	PrefixSecret = "sec:"
	// PrefixVariable refers to an ordinary value, which a read returns as it is.
	PrefixVariable = "var:"
)

// Collection is the set of values a reference names.
type Collection string

const (
	// CollectionSecret holds credentials.
	CollectionSecret Collection = "secrets"
	// CollectionVariable holds ordinary values.
	CollectionVariable Collection = "variables"
)

// Secret builds a reference to a credential.
func Secret(name string) string { return PrefixSecret + name }

// Variable builds a reference to an ordinary value.
func Variable(name string) string { return PrefixVariable + name }

// Parse splits a stored value into the collection it names and the name within it.
//
// A value carrying neither prefix is not a reference: it is the value itself, and is returned
// unchanged so a caller can pass anything through.
func Parse(stored string) (collection Collection, name string, isReference bool) {
	switch {
	case strings.HasPrefix(stored, PrefixSecret):
		return CollectionSecret, strings.TrimPrefix(stored, PrefixSecret), true
	case strings.HasPrefix(stored, PrefixVariable):
		return CollectionVariable, strings.TrimPrefix(stored, PrefixVariable), true
	default:
		return "", stored, false
	}
}

// Is reports whether a stored value is a reference to either collection.
func Is(stored string) bool {
	_, _, isReference := Parse(stored)
	return isReference
}

// IsSecret reports whether a stored value refers to a credential.
func IsSecret(stored string) bool {
	collection, _, isReference := Parse(stored)
	return isReference && collection == CollectionSecret
}

// Name returns the name a reference points at, without its prefix. A value that is not a reference
// is returned unchanged.
func Name(stored string) string {
	_, name, _ := Parse(stored)
	return name
}
