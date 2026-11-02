// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package valueref

import "testing"

// Building a reference and reading it back gives the collection and the name it was built from.
func TestAReferenceRoundTrips(t *testing.T) {
	for _, test := range []struct {
		name       string
		stored     string
		collection Collection
		want       string
	}{
		{"a secret", SecretReference("API_KEY"), CollectionSecret, "API_KEY"},
		{"a variable", VariableReference("HOST"), CollectionVariable, "HOST"},
	} {
		t.Run(test.name, func(t *testing.T) {
			collection, name, isReference := ParseReference(test.stored)
			if !isReference {
				t.Fatalf("%q was not read back as a reference", test.stored)
			}
			if collection != test.collection {
				t.Errorf("collection: got %q, want %q", collection, test.collection)
			}
			if name != test.want {
				t.Errorf("name: got %q, want %q", name, test.want)
			}
		})
	}
}

// A value carrying neither prefix is the value itself, and is handed back unchanged so a caller can
// pass anything through.
func TestAPlainValueIsNotAReference(t *testing.T) {
	const stored = "https://app.test:8090"

	collection, name, isReference := ParseReference(stored)
	if isReference {
		t.Fatalf("%q was read as a reference", stored)
	}
	if collection != "" {
		t.Errorf("a plain value named a collection: %q", collection)
	}
	if name != stored {
		t.Errorf("a plain value was altered: got %q, want %q", name, stored)
	}
	if IsReference(stored) {
		t.Error("IsReference reported a plain value as a reference")
	}
	if ReferencedName(stored) != stored {
		t.Errorf("ReferencedName altered a plain value: %q", ReferencedName(stored))
	}
}

// Telling a credential apart from an ordinary value is what the prefix is for: a data plane returns
// a variable's value to a read and never a secret's, so this is what keeps a credential out of one.
func TestOnlyASecretReferenceIsACredential(t *testing.T) {
	for stored, want := range map[string]bool{
		SecretReference("API_KEY"):   true,
		VariableReference("API_KEY"): false,
		"API_KEY":                    false,
		"":                           false,
	} {
		if got := IsSecretReference(stored); got != want {
			t.Errorf("IsSecretReference(%q): got %v, want %v", stored, got, want)
		}
	}
}

// The prefix is stripped from a reference and left alone otherwise, which is what lets a caller
// report a credential under the name it is actually held by.
func TestReferencedNameStripsThePrefix(t *testing.T) {
	for stored, want := range map[string]string{
		SecretReference("API_KEY"): "API_KEY",
		VariableReference("HOST"):  "HOST",
		"HOST":                     "HOST",
	} {
		if got := ReferencedName(stored); got != want {
			t.Errorf("ReferencedName(%q): got %q, want %q", stored, got, want)
		}
	}
}

// Only a reference whose name has the generated form counts as one. A credential that begins with a
// prefix by coincidence is a value, which is what keeps it from being mistaken for a reference.
func TestOnlyAWellFormedReferenceIsTrusted(t *testing.T) {
	for stored, want := range map[string]bool{
		"sec:APPLICATION_MY_APP_CLIENT_SECRET": true,
		"var:_1_HOST":                          true,
		"sec:a1b2-c3d4e5":                      false,
		"var:lowercase":                        false,
		"sec:":                                 false,
		"sec:1_LEADING_DIGIT":                  false,
		"https://app.test":                     false,
	} {
		if got := IsWellFormedReference(stored); got != want {
			t.Errorf("IsWellFormedReference(%q) = %v, want %v", stored, got, want)
		}
	}
}
