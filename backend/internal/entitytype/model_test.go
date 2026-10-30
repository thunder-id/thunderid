// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestTypeCategory_IsValid() {
	s.True(TypeCategoryUser.IsValid())
	s.True(TypeCategoryAgent.IsValid())
	s.True(TypeCategoryOrganization.IsValid())
	s.False(TypeCategory("app").IsValid())
	s.False(TypeCategory("").IsValid())
}
