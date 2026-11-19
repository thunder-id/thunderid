// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import "encoding/json"

// Predicate is a condition: a leaf comparison, an all or any group, or a negation.
type Predicate struct {
	Op      string
	Signal  string
	Value   *Literal
	Ref     string
	CI      bool
	Every   bool
	Options map[string]any
	Of      []*Predicate
}

// MarshalJSON encodes the predicate in its canonical JSON form: a group's children as an array,
// a negation's child as an object, and a leaf's fields in a fixed order.
func (p *Predicate) MarshalJSON() ([]byte, error) {
	switch p.Op {
	case opAll, opAny:
		return json.Marshal(struct {
			Op string       `json:"op"`
			Of []*Predicate `json:"of"`
		}{p.Op, p.Of})
	case opNot:
		if len(p.Of) != 1 {
			return nil, errNegationArity
		}
		return json.Marshal(struct {
			Op string     `json:"op"`
			Of *Predicate `json:"of"`
		}{p.Op, p.Of[0]})
	}
	return json.Marshal(struct {
		Op      string         `json:"op"`
		Signal  string         `json:"signal"`
		Value   *Literal       `json:"value,omitempty"`
		Ref     string         `json:"ref,omitempty"`
		CI      bool           `json:"ci,omitempty"`
		Every   bool           `json:"every,omitempty"`
		Options map[string]any `json:"options,omitempty"`
	}{p.Op, p.Signal, p.Value, p.Ref, p.CI, p.Every, p.Options})
}

// isGroup reports whether the predicate is an all or any group.
func (p *Predicate) isGroup() bool {
	return p.Op == opAll || p.Op == opAny
}

// literalKind is the JSON type a literal value was written as.
type literalKind uint8

// Literal value kinds.
const (
	literalString literalKind = iota
	literalNumber
	literalBool
)

// literalValue is one value of a literal, in its canonical text form.
type literalValue struct {
	text string
	kind literalKind
}

// Literal is the value a leaf compares with: one value, or a list matched when any element matches.
type Literal struct {
	values []literalValue
	list   bool
}

// Atoms returns the canonical text form of each value of the literal.
func (l Literal) Atoms() []string {
	atoms := make([]string, len(l.values))
	for i, v := range l.values {
		atoms[i] = v.text
	}
	return atoms
}

// IsList reports whether the literal was written as a list.
func (l Literal) IsList() bool {
	return l.list
}

// MarshalJSON encodes the literal with each value in its original JSON type.
func (l Literal) MarshalJSON() ([]byte, error) {
	encoded := make([]json.RawMessage, len(l.values))
	for i, v := range l.values {
		raw, err := v.marshal()
		if err != nil {
			return nil, err
		}
		encoded[i] = raw
	}
	if !l.list && len(encoded) == 1 {
		return encoded[0], nil
	}
	return json.Marshal(encoded)
}

// marshal encodes one literal value as its original JSON type.
func (v literalValue) marshal() (json.RawMessage, error) {
	switch v.kind {
	case literalNumber, literalBool:
		return json.RawMessage(v.text), nil
	}
	return json.Marshal(v.text)
}
