// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"math/bits"
	"strings"
)

// comparatorID identifies a comparator. The inferable comparators come first, in inference order,
// so the lowest set bit of a mask is the comparator to infer.
type comparatorID uint8

// Comparators.
const (
	cmpInstant comparatorID = iota
	cmpDuration
	cmpNumber
	cmpBoolean
	cmpText
	cmpScale
	cmpNone
)

// comparatorMask is a set of inferable comparators, one bit each.
type comparatorMask uint8

// ability is what an operator needs a comparator to do.
type ability uint8

// Abilities.
const (
	abilityEquality ability = 1 << iota
	abilityOrdering
	abilityText
)

// comparatorInfo describes one comparator.
type comparatorInfo struct {
	name      ComparatorName
	inferred  bool
	abilities ability
}

// comparators describes every comparator, indexed by comparatorID.
var comparators = [...]comparatorInfo{
	cmpInstant:  {ComparatorInstant, true, abilityEquality | abilityOrdering},
	cmpDuration: {ComparatorDuration, true, abilityEquality | abilityOrdering},
	cmpNumber:   {ComparatorNumber, true, abilityEquality | abilityOrdering},
	cmpBoolean:  {ComparatorBoolean, true, abilityEquality},
	cmpText:     {ComparatorText, true, abilityEquality | abilityText},
	cmpScale:    {ComparatorScale, false, abilityEquality | abilityOrdering},
}

// maskOf returns the mask holding one comparator.
func maskOf(c comparatorID) comparatorMask {
	return 1 << c
}

// inferableWith returns the inferable comparators that offer the ability.
func inferableWith(need ability) comparatorMask {
	var m comparatorMask
	for c, info := range comparators {
		if info.inferred && info.abilities&need == need {
			m |= maskOf(comparatorID(c))
		}
	}
	return m
}

// comparatorByName returns the comparator with the given name, or cmpNone.
func comparatorByName(name ComparatorName) comparatorID {
	for c, info := range comparators {
		if info.name == name {
			return comparatorID(c)
		}
	}
	return cmpNone
}

// offers reports whether the comparator offers the ability.
func (c comparatorID) offers(need ability) bool {
	return c < cmpNone && comparators[c].abilities&need == need
}

// displayName returns the comparator's name, or an empty name for cmpNone.
func (c comparatorID) displayName() ComparatorName {
	if c >= cmpNone {
		return ""
	}
	return comparators[c].name
}

// comparison is how one leaf reads its values: a declared comparator, or inference among those that
// offer the ability its operator needs.
type comparison struct {
	pinned  comparatorID
	scale   map[string]int
	allowed comparatorMask
	need    ability
	ci      bool
}

// pick returns the comparator for one pair of values, or cmpNone when no comparator can compare them.
func (c *comparison) pick(a, b *atom) comparatorID {
	switch c.pinned {
	case cmpNone:
		m := a.classes & b.classes & c.allowed
		if m == 0 {
			return cmpNone
		}
		return comparatorID(bits.TrailingZeros8(uint8(m))) // #nosec G115 -- a uint8 has at most 8 trailing zeros.
	case cmpScale:
		if !c.pinned.offers(c.need) || !c.inScale(a) || !c.inScale(b) {
			return cmpNone
		}
		return cmpScale
	}
	if !c.pinned.offers(c.need) || a.classes&b.classes&maskOf(c.pinned) == 0 {
		return cmpNone
	}
	return c.pinned
}

// recognizes reports whether the declared comparator can read the atom. Without a declared
// comparator, every atom is readable.
func (c *comparison) recognizes(a *atom) bool {
	switch c.pinned {
	case cmpNone:
		return true
	case cmpScale:
		return c.inScale(a)
	}
	return a.classes&maskOf(c.pinned) != 0
}

// inScale reports whether the atom is a member of the declared scale.
func (c *comparison) inScale(a *atom) bool {
	_, ok := c.scale[a.text]
	return ok
}

// equal reports whether two atoms are equal under the comparator.
func (c *comparison) equal(cmp comparatorID, a, b *atom) bool {
	switch cmp {
	case cmpText:
		if c.ci {
			return a.lower == b.lower
		}
		return a.text == b.text
	case cmpScale:
		return c.scale[a.text] == c.scale[b.text]
	case cmpBoolean:
		return a.text == b.text
	}
	return c.order(cmp, a, b) == 0
}

// order returns -1, 0 or 1 as a is before, equal to or after b under an ordering comparator.
func (c *comparison) order(cmp comparatorID, a, b *atom) int {
	switch cmp {
	case cmpNumber:
		return compareDecimal(a.number, b.number)
	case cmpInstant:
		return a.instant.Compare(b.instant)
	case cmpDuration:
		return sign(int(a.period.nominalSeconds() - b.period.nominalSeconds()))
	case cmpScale:
		return sign(c.scale[a.text] - c.scale[b.text])
	}
	return 0
}

// textOf returns the text the atom is matched by, folded when the comparison ignores case.
func (c *comparison) textOf(a *atom) string {
	if c.ci {
		return a.lower
	}
	return a.text
}

// matchText applies a text operator to two strings.
func matchText(op, s, sub string) bool {
	switch op {
	case opStartsWith:
		return strings.HasPrefix(s, sub)
	case opEndsWith:
		return strings.HasSuffix(s, sub)
	}
	return strings.Contains(s, sub)
}
