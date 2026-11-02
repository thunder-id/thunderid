// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TextTestSuite struct {
	suite.Suite
}

func TestTextTestSuite(t *testing.T) {
	suite.Run(t, new(TextTestSuite))
}

// Text is safe to show unless it carries a control character other than whitespace or a bidirectional
// formatting character.
func (s *TextTestSuite) TestIsSafeText() {
	for _, text := range []string{"", "Pay {{amount}}", "line one\nline two\ttabbed", "தமிழ் payment"} {
		s.True(IsSafeText(text), text)
	}
	for _, text := range []string{"pay\x00", "pay\x1b[31m", "pay\u0085", "pay \u202eexe", "pay\u2066", "pay\u200f"} {
		s.False(IsSafeText(text), text)
	}
}
