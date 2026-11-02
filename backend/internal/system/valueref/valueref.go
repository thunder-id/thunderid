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

import (
	"regexp"
	"strings"
)

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

// SecretReference builds a reference to a credential.
func SecretReference(name string) string { return PrefixSecret + name }

// VariableReference builds a reference to an ordinary value.
func VariableReference(name string) string { return PrefixVariable + name }

// ParseReference splits a stored value into the collection it names and the name within it.
//
// A value carrying neither prefix is not a reference: it is the value itself, and is returned
// unchanged so a caller can pass anything through.
func ParseReference(stored string) (collection Collection, name string, isReference bool) {
	switch {
	case strings.HasPrefix(stored, PrefixSecret):
		return CollectionSecret, strings.TrimPrefix(stored, PrefixSecret), true
	case strings.HasPrefix(stored, PrefixVariable):
		return CollectionVariable, strings.TrimPrefix(stored, PrefixVariable), true
	default:
		return "", stored, false
	}
}

// IsReference reports whether a stored value is a reference to either collection.
func IsReference(stored string) bool {
	_, _, isReference := ParseReference(stored)
	return isReference
}

// IsSecretReference reports whether a stored value refers to a credential.
func IsSecretReference(stored string) bool {
	collection, _, isReference := ParseReference(stored)
	return isReference && collection == CollectionSecret
}

// ReferencedName returns the name a reference points at, without its prefix. A value that is not a
// reference is returned unchanged.
func ReferencedName(stored string) string {
	_, name, _ := ParseReference(stored)
	return name
}

// IsWellFormedReference reports whether stored is a reference whose name has the form every
// generated name has: upper-case letters, digits and underscores, not starting with a digit.
//
// A field holds either a value or a reference, and only the form tells them apart. A credential is
// free-form, so one could begin with a prefix by coincidence, and a check on the prefix alone would
// then take the credential for a reference and write it out as one. Requiring the name's form narrows
// that to a credential that is itself a prefix followed by such a name.
func IsWellFormedReference(stored string) bool {
	_, name, isReference := ParseReference(stored)
	return isReference && wellFormedName.MatchString(name)
}

// wellFormedName is the form of every name a reference is generated with.
var wellFormedName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
