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

// Only an owner managed in another identity system and named by an email address or a mobile number has a
// contact, compared in its canonical form.
func (s *UtilsTestSuite) TestAgentOwnerContact() {
	for name, tc := range map[string]struct {
		systemAttributes, value, contactType string
	}{
		"email": {`{"owner":{"id":" a@EXAMPLE.com ","idType":"email","isLocal":false}}`,
			"a@example.com", "email"},
		"mobile": {`{"owner":{"id":"+94771234567","idType":"mobile","isLocal":false}}`,
			"+94771234567", "mobile"},
		"local owner":      {`{"owner":{"id":"user-1","idType":"uuid","isLocal":true}}`, "", ""},
		"owner by default": {`{"owner":{"id":"user-1"}}`, "", ""},
		"external subject": {`{"owner":{"id":"auth0|1","idType":"uuid","isLocal":false}}`, "", ""},
		"blank id":         {`{"owner":{"id":"  ","idType":"email","isLocal":false}}`, "", ""},
		"legacy string":    {`{"owner":"user-1"}`, "", ""},
		"no owner":         {`{"clientId":"c"}`, "", ""},
		"empty":            {``, "", ""},
		"not json":         {`{`, "", ""},
	} {
		s.Run(name, func() {
			value, contactType := AgentOwnerContact(json.RawMessage(tc.systemAttributes))

			s.Equal(tc.value, value)
			s.Equal(tc.contactType, contactType)
		})
	}
}
