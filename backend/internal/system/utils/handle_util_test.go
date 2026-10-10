// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HandleUtilTestSuite struct {
	suite.Suite
}

func TestHandleUtilSuite(t *testing.T) {
	suite.Run(t, new(HandleUtilTestSuite))
}

func (suite *HandleUtilTestSuite) TestIsValidHandle_Valid() {
	validHandles := []string{
		"a",
		"1",
		"ab",
		"person",
		"customer-accounts",
		"customer_accounts",
		"a-b_c",
		"user2",
		"2fa-users",
		"a--b",
		"a__b",
	}

	for _, handle := range validHandles {
		suite.True(IsValidHandle(handle), "expected %q to be valid", handle)
	}
}

func (suite *HandleUtilTestSuite) TestIsValidHandle_Invalid() {
	invalidHandles := []string{
		"",
		"-",
		"_",
		"-person",
		"person-",
		"_person",
		"person_",
		"Person",
		"PERSON",
		"per son",
		" person",
		"person ",
		"per.son",
		"per/son",
		"per@son",
		"person\n",
		"pérson",
	}

	for _, handle := range invalidHandles {
		suite.False(IsValidHandle(handle), "expected %q to be invalid", handle)
	}
}

func (suite *HandleUtilTestSuite) TestIsValidHandle_MaxLength() {
	suite.True(IsValidHandle(strings.Repeat("a", maxHandleLength)))
	suite.False(IsValidHandle(strings.Repeat("a", maxHandleLength+1)))
}
