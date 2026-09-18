// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package variablestore holds the named values this deployment keeps for configuration to refer to
// instead of carrying inline.
//
// There are two collections, and the only difference that matters is what may be read back. A
// variable's value is returned; a secret's never is. That asymmetry is enforced by the types here
// rather than by remembering to omit a field: Secret has no value at all, so no handler can return
// one by accident.
package variablestore

import "regexp"

// nameFormat is what a name may look like.
//
// Narrow on purpose. These names are substituted into configuration as template variables, and a
// name needing escaping is a name that will eventually be got wrong by something that forgets to.
var nameFormat = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

const (
	maxNameLength        = 255
	maxValueLength       = 8192
	maxDescriptionLength = 1000
	// maxNamesPerQuery bounds a names= lookup, so one request cannot ask for unbounded work.
	maxNamesPerQuery = 100
)

// Variable is a named value whose contents are readable.
type Variable struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// Secret is a named credential. It carries no value field: the value travels inbound only, so there
// is nothing here for a response to disclose.
type Secret struct {
	Name string `json:"name"`
	// Exists is always true on anything returned. It is present so a names= lookup has one shape
	// whether or not each name was found.
	Exists      bool   `json:"exists"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// VariableRequest creates a variable.
type VariableRequest struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// VariableUpdateRequest replaces a variable's value.
type VariableUpdateRequest struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// SecretRequest stores a secret.
type SecretRequest struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// SecretUpdateRequest replaces a secret's value, which is how one is rotated.
type SecretUpdateRequest struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// Link is a pagination link.
type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// VariableListResponse is a page of variables.
type VariableListResponse struct {
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	Count        int        `json:"count"`
	Variables    []Variable `json:"variables"`
	Links        []Link     `json:"links"`
}

// SecretListResponse is a page of secrets, by name.
type SecretListResponse struct {
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	Count        int      `json:"count"`
	Secrets      []Secret `json:"secrets"`
	Links        []Link   `json:"links"`
}

// listQuery is a resolved list request: what to return and how much of it.
type listQuery struct {
	limit  int
	offset int
	// names, when set, restricts the result to these. A name with nothing stored is absent from the
	// result rather than an error: the caller is asking which of these exist.
	names []string
	// namePrefix and nameEquals come from the filter expression, at most one of them set.
	namePrefix string
	nameEquals string
}
