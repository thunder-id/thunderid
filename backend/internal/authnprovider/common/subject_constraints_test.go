// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type SubjectTypeConstraintsTestSuite struct {
	suite.Suite
}

func TestSubjectTypeConstraintsTestSuite(t *testing.T) {
	suite.Run(t, new(SubjectTypeConstraintsTestSuite))
}

func (suite *SubjectTypeConstraintsTestSuite) TestPermitsSubject() {
	constraints := SubjectTypeConstraints{
		AllowedUserTypes:  []string{"customer"},
		AllowedAgentTypes: []string{"default"},
	}

	testCases := []struct {
		name        string
		constraints SubjectTypeConstraints
		category    providers.EntityCategory
		entityType  string
		expected    bool
	}{
		{"any user type", constraints, providers.EntityCategoryUser, "employee", true},
		{"listed agent type", constraints, providers.EntityCategoryAgent, "default", true},
		{"unlisted agent type", constraints, providers.EntityCategoryAgent, "privileged", false},
		{
			"empty agent list accepts no agent",
			SubjectTypeConstraints{}, providers.EntityCategoryAgent, "default", false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			assert.Equal(suite.T(), tc.expected,
				tc.constraints.PermitsSubject(tc.category, tc.entityType))
		})
	}
}

func (suite *SubjectTypeConstraintsTestSuite) TestContextRoundTrip() {
	in := SubjectTypeConstraints{
		AllowedUserTypes:  []string{"customer"},
		AllowedAgentTypes: []string{"default"},
	}

	out, ok := SubjectTypeConstraintsFrom(WithSubjectTypeConstraints(context.Background(), in))

	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), in, out)
}

func (suite *SubjectTypeConstraintsTestSuite) TestContextWithoutConstraints() {
	out, ok := SubjectTypeConstraintsFrom(context.Background())

	assert.False(suite.T(), ok)
	assert.Equal(suite.T(), SubjectTypeConstraints{}, out)
}
