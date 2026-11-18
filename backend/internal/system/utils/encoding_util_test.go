// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type EncodingUtilTestSuite struct {
	suite.Suite
}

func TestEncodingUtilTestSuite(t *testing.T) {
	suite.Run(t, new(EncodingUtilTestSuite))
}

func (suite *EncodingUtilTestSuite) TestEncodeURLSafeID() {
	testCases := []struct {
		name     string
		id       string
		expected string
	}{
		{"LowercaseUUID", "019a8c5e-2f41-7b3d-a9e6-5c0d1f7b3e28", "AZqMXi9Bez2p5lwNH3s-KA"},
		{"UppercaseUUID", "019A8C5E-2F41-7B3D-A9E6-5C0D1F7B3E28", "~MDE5QThDNUUtMkY0MS03QjNELUE5RTYtNUMwRDFGN0IzRTI4"},
		{"NonUUID", "customer-portal1", "~Y3VzdG9tZXItcG9ydGFsMQ"},
		{"Empty", "", ""},
		{"InvalidUTF8", string([]byte{0xff}), ""},
		{"NULCharacter", "app\x00id", ""},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			assert.Equal(suite.T(), tc.expected, EncodeURLSafeID(tc.id))
		})
	}
}

func (suite *EncodingUtilTestSuite) TestDecodeURLSafeID_RoundTrip() {
	for _, id := range []string{
		"019a8c5e-2f41-7b3d-a9e6-5c0d1f7b3e28",
		"019A8C5E-2F41-7B3D-A9E6-5C0D1F7B3E28",
		"customer-portal1",
		"my-app",
	} {
		decoded, err := DecodeURLSafeID(EncodeURLSafeID(id))
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), id, decoded)
	}
}

func (suite *EncodingUtilTestSuite) TestDecodeURLSafeID_Invalid() {
	for _, value := range []string{
		"",
		"~",
		"not*base64",
		"~not*base64",
		"bXktYXBw",
		"~_w",
		// A NUL character.
		"~AA",
		// Non-zero trailing bits decode to the same UUID as AZqMXi9Bez2p5lwNH3s-KA.
		"AZqMXi9Bez2p5lwNH3s-KB",
		// A lowercase UUID in the string form, which the encoder never produces for it.
		"~MDE5YThjNWUtMmY0MS03YjNkLWE5ZTYtNWMwZDFmN2IzZTI4",
	} {
		_, err := DecodeURLSafeID(value)
		assert.Error(suite.T(), err, value)
	}
}
