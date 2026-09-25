// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ErrorConstantsTestSuite groups the tests in error_constants_test.go.
type ErrorConstantsTestSuite struct {
	suite.Suite
}

// TestErrorConstantsTestSuite runs ErrorConstantsTestSuite.
func TestErrorConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(ErrorConstantsTestSuite))
}

// TestNewMissingRequiredAttributesError tests New Missing Required Attributes Error.
func (suite *ErrorConstantsTestSuite) TestNewMissingRequiredAttributesError() {
	t := suite.T()
	svcErr := NewMissingRequiredAttributesError("employee", []string{"department", "employee_id"})

	require.Equal(t, ErrorSchemaValidationFailed.Code, svcErr.Code)
	require.Equal(t, ErrorSchemaValidationFailed.Type, svcErr.Type)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, `"employee"`)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, "department, employee_id")
}

// TestNewUndeclaredAttributesError tests New Undeclared Attributes Error.
func (suite *ErrorConstantsTestSuite) TestNewUndeclaredAttributesError() {
	t := suite.T()
	svcErr := NewUndeclaredAttributesError("employee", []string{"extra1", "extra2"})

	require.Equal(t, ErrorSchemaValidationFailed.Code, svcErr.Code)
	require.Equal(t, ErrorSchemaValidationFailed.Type, svcErr.Type)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, `"employee"`)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, "extra1, extra2")
}
