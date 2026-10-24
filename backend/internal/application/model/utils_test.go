// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
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

func (s *UtilsTestSuite) TestIsFlowNativeGrantSet() {
	tokenExchange := string(providers.GrantTypeTokenExchange)
	cases := []struct {
		name       string
		grantTypes []string
		want       bool
	}{
		{name: "no grants", grantTypes: nil},
		{name: "token exchange only", grantTypes: []string{tokenExchange}, want: true},
		{name: "client_credentials and token exchange", grantTypes: []string{"client_credentials", tokenExchange},
			want: true},
		{name: "authorization_code and token exchange", grantTypes: []string{"authorization_code", tokenExchange}},
		{name: "client_credentials only", grantTypes: []string{"client_credentials"}},
		{name: "jwt-bearer only", grantTypes: []string{string(providers.GrantTypeJWTBearer)}},
		{name: "CIBA only", grantTypes: []string{string(providers.GrantTypeCIBA)}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, IsFlowNativeGrantSet(tc.grantTypes))
		})
	}
}

func (s *UtilsTestSuite) TestIsFlowNativeGrantSet_TypedGrantTypes() {
	s.True(IsFlowNativeGrantSet([]providers.GrantType{
		providers.GrantTypeClientCredentials, providers.GrantTypeTokenExchange,
	}))
	s.False(IsFlowNativeGrantSet([]providers.GrantType{providers.GrantTypeJWTBearer}))
}
