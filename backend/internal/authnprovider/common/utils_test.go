// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestValidUntilOf() {
	cases := []struct {
		name        string
		attrs       string
		wantBounded bool
		want        time.Time
		wantErr     bool
	}{
		{name: "no attributes", attrs: ""},
		{name: "absent", attrs: `{"owner":"o-1"}`},
		{name: "unbounded sentinel", attrs: `{"validUntil":"9999-12-31T23:59:59Z"}`},
		{name: "bounded", attrs: `{"validUntil":"2027-03-31T05:30:00+05:30"}`, wantBounded: true,
			want: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC)},
		{name: "unparsable blob", attrs: `not json`, wantErr: true},
		{name: "not a string", attrs: `{"validUntil":12}`, wantErr: true},
		{name: "not RFC 3339", attrs: `{"validUntil":"2027-03-31"}`, wantErr: true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, bounded, err := ValidUntilOf(json.RawMessage(tc.attrs))
			if tc.wantErr {
				s.Require().Error(err)
				s.False(bounded, "a corrupt value never reads as a bound to skip")
				return
			}
			s.Require().NoError(err)
			s.Equal(tc.wantBounded, bounded)
			s.True(tc.want.Equal(got))
		})
	}
}

func (s *UtilsTestSuite) TestAuthenticationMethodOf() {
	cases := []struct {
		name        string
		credentials map[string]interface{}
		method      providers.AuthenticationMethod
	}{
		{"password", map[string]interface{}{"password": "p"}, providers.AuthenticationMethodCredential},
		{"other schema secrets", map[string]interface{}{"pin": "1234", "secretAnswer": "blue"},
			providers.AuthenticationMethodCredential},
		{"a key named after the account-wide entry", map[string]interface{}{"account": "x"},
			providers.AuthenticationMethodCredential},
		{"a system credential beside a password", map[string]interface{}{CredentialTypeClientSecret: "s",
			"password": "p"}, providers.AuthenticationMethodCredential},
		{"passkey", map[string]interface{}{CredentialTypePasskey: "x"}, providers.AuthenticationMethodPasskey},
		{"otp", map[string]interface{}{CredentialTypeOTP: "x"}, providers.AuthenticationMethodOTP},
		{"federated", map[string]interface{}{CredentialTypeFederated: "x"}, providers.AuthenticationMethodFederated},
		{"magic link", map[string]interface{}{CredentialTypeMagicLink: "x"}, providers.AuthenticationMethodMagicLink},
		{"openid4vp", map[string]interface{}{CredentialTypeOpenID4VP: "x"}, providers.AuthenticationMethodOpenID4VP},
		{"a provisioned entity", map[string]interface{}{CredentialTypeProvisionedEntityID: "x"},
			providers.AuthenticationMethodFederated},
		{"a provisioned entity beside a passkey", map[string]interface{}{CredentialTypeProvisionedEntityID: "x",
			CredentialTypePasskey: "x"}, providers.AuthenticationMethodFederated},
		{"a passkey beside a passcode", map[string]interface{}{CredentialTypeOTP: "x", CredentialTypePasskey: "x"},
			providers.AuthenticationMethodPasskey},
		{"a passcode beside a password", map[string]interface{}{CredentialTypeOTP: "x", "password": "p"},
			providers.AuthenticationMethodOTP},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.method, AuthenticationMethodOf(tc.credentials))
		})
	}
	for _, credentialType := range SystemCredentialTypes {
		s.Equal(providers.AuthenticationMethodSystemCredential,
			AuthenticationMethodOf(map[string]interface{}{credentialType: "s"}), credentialType)
	}
}

func (s *UtilsTestSuite) TestAuthenticationMethodOf_NamesEveryInternalCredentialType() {
	for _, credentialType := range InternalCredentialTypes {
		method := AuthenticationMethodOf(map[string]interface{}{credentialType: "x"})

		s.NotEqual(providers.AuthenticationMethodCredential, method, credentialType)
		s.NotEqual(providers.AuthenticationMethodSystemCredential, method, credentialType)
	}
}
