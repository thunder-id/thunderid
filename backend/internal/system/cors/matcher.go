/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cors

import "regexp"

// Matcher evaluates a parsed origin against a fixed set of compiled rules.
// It is constructed once at server start from the deployment configuration
// and is safe for concurrent use after construction (no internal mutation).
//
// Internally the matcher partitions rules into a canonical-keyed map (for
// O(1) literal lookups) and a regex slice (iterated only when the literal
// lookup misses). The "null" origin is tracked separately because it has no
// canonical form.
type Matcher struct {
	literals    map[string]struct{}
	regexes     []*regexp.Regexp
	nullAllowed bool
	size        int
}

// newMatcher constructs a Matcher over the given compiled rules. Literal and
// regex rules are partitioned; null literals are tracked via nullAllowed. A
// nil or empty slice yields a matcher that rejects every origin.
func newMatcher(rules []originRule) *Matcher {
	m := &Matcher{
		literals: make(map[string]struct{}),
		size:     len(rules),
	}
	for _, rule := range rules {
		switch r := rule.(type) {
		case literalRule:
			if r.isNull {
				m.nullAllowed = true
				continue
			}
			m.literals[r.canonical] = struct{}{}
		case regexRule:
			m.regexes = append(m.regexes, r.re)
		}
	}
	return m
}

// combine returns a new matcher that accepts an origin if either input does, folding the once-compiled
// read-only matcher with the writable one. Neither input is mutated — the literal map and regex slice are
// freshly allocated — so the shared read-only matcher stays safe for concurrent use. combine(a, nil) == a
// and combine(nil, b) == b. Literals are de-duplicated via the map; regexes are concatenated as-is (a
// cross-layer duplicate is just evaluated twice on a miss, which is harmless).
func combine(a, b *Matcher) *Matcher {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := &Matcher{
		literals:    make(map[string]struct{}, len(a.literals)+len(b.literals)),
		regexes:     make([]*regexp.Regexp, 0, len(a.regexes)+len(b.regexes)),
		nullAllowed: a.nullAllowed || b.nullAllowed,
	}
	for k := range a.literals {
		out.literals[k] = struct{}{}
	}
	for k := range b.literals {
		out.literals[k] = struct{}{}
	}
	out.regexes = append(out.regexes, a.regexes...)
	out.regexes = append(out.regexes, b.regexes...)
	out.size = len(out.literals) + len(out.regexes)
	if out.nullAllowed {
		out.size++
	}
	return out
}

// Match evaluates the parsed origin against the configured rules and returns
// the verbatim raw origin as the echo target on a hit. Literal lookup runs
// first against the canonical-key map; if it misses, regex rules are tested
// against the raw header in declaration order.
//
// Callers must construct parsed via ParseOrigin so parsed.Canonical is
// populated. The literal map is keyed on the canonical form and a missing
// Canonical will silently miss it; the IsNull short-circuit is the only
// hand-buildable case.
func (m *Matcher) Match(parsed ParseResult) (allow bool, echo string) {
	if m == nil {
		return false, ""
	}
	if parsed.IsNull {
		if m.nullAllowed {
			return true, parsed.Raw
		}
		return false, ""
	}
	if parsed.Canonical != "" {
		if _, ok := m.literals[parsed.Canonical]; ok {
			return true, parsed.Raw
		}
	}
	for _, re := range m.regexes {
		if re.MatchString(parsed.Raw) {
			return true, parsed.Raw
		}
	}
	return false, ""
}

// Size reports the total number of rules the matcher holds.
func (m *Matcher) Size() int {
	if m == nil {
		return 0
	}
	return m.size
}

// LiteralCount reports the number of literal rules (including any "null"
// entry) held by the matcher.
func (m *Matcher) LiteralCount() int {
	if m == nil {
		return 0
	}
	n := len(m.literals)
	if m.nullAllowed {
		n++
	}
	return n
}

// RegexCount reports the number of regex rules held by the matcher.
func (m *Matcher) RegexCount() int {
	if m == nil {
		return 0
	}
	return len(m.regexes)
}
