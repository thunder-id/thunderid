// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestReadRecordedAttributeVerificationsRejectsMalformedStoredMetadata() {
	for _, raw := range []string{`{"attributeVerification":null}`, `{"attributeVerification":[]}`,
		`{"attributeVerification":{"email":"t"}}`, `{"attributeVerification":{"email":{"verifiedAt":5}}}`} {
		_, err := ReadRecordedAttributeVerifications(json.RawMessage(raw))
		s.Require().Error(err, raw)
	}
	// An entry without a valid verification time records nothing.
	entries, err := ReadRecordedAttributeVerifications(json.RawMessage(
		`{"attributeVerification":{"email":{},"mobile":null,"workEmail":{"revision":1},` +
			`"homeEmail":{"verifiedAt":"forged"}}}`))
	s.Require().NoError(err)
	s.Require().Empty(entries)
}

func (s *UtilsTestSuite) TestReadRecordedAttributeVerifications() {
	loaded := providers.Entity{SystemAttributes: json.RawMessage(`{"attributeVerification":{` +
		`"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
		`"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
		`"nickname":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
		`"homeEmail":{"verifiedAt":"forged"},` +
		`"removedEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)}

	entries, err := ReadRecordedAttributeVerifications(loaded.SystemAttributes)

	s.Require().NoError(err)
	s.Equal(map[string]providers.VerifiedAttribute{
		"workEmail":    {VerifiedAt: "2026-10-15T00:00:00Z"},
		"mobile":       {VerifiedAt: "2026-10-15T00:00:00Z"},
		"nickname":     {VerifiedAt: "2026-10-15T00:00:00Z"},
		"removedEmail": {VerifiedAt: "2026-10-15T00:00:00Z"},
	}, entries)
}

func (s *UtilsTestSuite) TestReadRecordedAttributeVerificationsWithoutMetadata() {
	for _, raw := range []string{"", `null`, `{}`, `{"attributeVerification":{}}`} {
		entries, err := ReadRecordedAttributeVerifications(json.RawMessage(raw))

		s.Require().NoError(err, raw)
		s.NotNil(entries, raw)
		s.Empty(entries, raw)
	}
}
