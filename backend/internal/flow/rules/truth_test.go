// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TruthTestSuite struct {
	suite.Suite
}

func TestTruthTestSuite(t *testing.T) {
	suite.Run(t, new(TruthTestSuite))
}

func (s *TruthTestSuite) TestText() {
	s.Equal("TRUE", TruthTrue.String())
	s.Equal("FALSE", TruthFalse.String())
	s.Equal("UNKNOWN", TruthUnknown.String())
	s.Equal("UNKNOWN", Truth(9).String())

	out, err := json.Marshal([]Truth{TruthTrue, TruthUnknown})
	s.NoError(err)
	s.Equal(`["TRUE","UNKNOWN"]`, string(out))
}

func (s *TruthTestSuite) TestKleeneTables() {
	f, u, t := TruthFalse, TruthUnknown, TruthTrue
	and := [3][3]Truth{{f, f, f}, {f, u, u}, {f, u, t}}
	or := [3][3]Truth{{f, u, t}, {u, u, t}, {t, t, t}}
	for a := f; a <= t; a++ {
		for b := f; b <= t; b++ {
			s.Equal(and[a][b], andTruth(a, b))
			s.Equal(or[a][b], orTruth(a, b))
		}
	}
	s.Equal(t, notTruth(f))
	s.Equal(u, notTruth(u))
	s.Equal(f, notTruth(t))
}
