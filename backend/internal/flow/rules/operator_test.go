// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type OperatorTestSuite struct {
	suite.Suite
}

func TestOperatorTestSuite(t *testing.T) {
	suite.Run(t, new(OperatorTestSuite))
}

func (s *OperatorTestSuite) TestOperatorByName() {
	spec, ok := operatorByName(opContains)
	s.True(ok)
	s.Equal(abilityText, spec.need)

	_, ok = operatorByName("matches")
	s.False(ok)
}

func (s *OperatorTestSuite) TestPairTests() {
	cases := []struct {
		name   string
		op     string
		pinned comparatorID
		ci     bool
		a, b   string
		want   Truth
		cmp    comparatorID
	}{
		{"eq numbers", opEq, cmpNone, false, "3.0", "3", TruthTrue, cmpNumber},
		{"eq text", opEq, cmpNone, false, "a", "b", TruthFalse, cmpText},
		{"gt numbers", opGt, cmpNone, false, "30", "18", TruthTrue, cmpNumber},
		{"gt equal", opGt, cmpNone, false, "18", "18", TruthFalse, cmpNumber},
		{"gte equal", opGte, cmpNone, false, "18", "18", TruthTrue, cmpNumber},
		{"gt words", opGt, cmpNone, false, "b", "a", TruthUnknown, cmpNone},
		{"gte words", opGte, cmpNone, false, "b", "a", TruthUnknown, cmpNone},
		{"starts with", opStartsWith, cmpNone, false, "alice@x", "alice", TruthTrue, cmpText},
		{"starts with case", opStartsWith, cmpNone, false, "Alice@x", "alice", TruthFalse, cmpText},
		{"starts with ignoring case", opStartsWith, cmpNone, true, "Alice@x", "alice", TruthTrue, cmpText},
		{"ends with", opEndsWith, cmpNone, false, "alice@x.com", "@x.com", TruthTrue, cmpText},
		{"contains", opContains, cmpNone, false, "alice@x.com", "@", TruthTrue, cmpText},
		{"contains on declared number", opContains, cmpNumber, false, "123", "2", TruthUnknown, cmpNone},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			spec, _ := operatorByName(tc.op)
			l := &leaf{op: tc.op, cmp: comparison{pinned: tc.pinned, allowed: inferableWith(spec.need),
				need: spec.need, ci: tc.ci}}
			a, b := newAtom(tc.a), newAtom(tc.b)

			got, c := spec.evaluateAt(l, &a, &b)

			s.Equal(tc.want, got)
			s.Equal(tc.cmp, c)
		})
	}
}

func (s *OperatorTestSuite) TestTruthOf() {
	s.Equal(TruthTrue, truthOf(true))
	s.Equal(TruthFalse, truthOf(false))
}
