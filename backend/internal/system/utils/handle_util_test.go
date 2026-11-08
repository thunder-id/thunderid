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

func (suite *HandleUtilTestSuite) TestGenerateHandle() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Words", input: "Northstar Logistics", expected: "northstar-logistics"},
		{name: "Punctuation and padding", input: "  ACME, Inc.  ", expected: "acme-inc"},
		{name: "Accents", input: "Café Société", expected: "cafe-societe"},
		{name: "Symbols and numbers", input: "R&D Team #42", expected: "r-d-team-42"},
		{name: "Underscore", input: "my_org", expected: "my-org"},
		{name: "Already a handle", input: "engineering", expected: "engineering"},
		{name: "Full-width characters", input: "ＡＢＣ", expected: "abc"},
		{name: "No usable characters", input: "株式会社 🚀", expected: ""},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			handle := GenerateHandle(tc.input)
			suite.Equal(tc.expected, handle)
			if handle != "" {
				suite.True(IsValidHandle(handle))
			}
		})
	}
}

func (suite *HandleUtilTestSuite) TestGenerateHandle_MaxLength() {
	suite.Equal(strings.Repeat("a", 100), GenerateHandle(strings.Repeat("a", 150)))

	handle := GenerateHandle(strings.Repeat("a", 99) + " b")
	suite.Equal(strings.Repeat("a", 99), handle)
	suite.True(IsValidHandle(handle))
}
