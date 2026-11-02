// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestSMSVendorName() {
	name, ok := smsVendorName(ncommon.NotificationProviderTypeTwilio)
	s.True(ok)
	s.Equal("twilio", name)
}

func (s *UtilsTestSuite) TestSMSVendorNameUnregisteredProviderReturnsFalse() {
	name, ok := smsVendorName(ncommon.NotificationProviderType("unregistered-provider"))
	s.False(ok)
	s.Empty(name)
}

func (s *UtilsTestSuite) TestEmailVendorName() {
	name, ok := emailVendorName(ncommon.NotificationProviderTypeSMTP)
	s.True(ok)
	s.Equal(emailSMTPVendorName, name)
}

// A message provider must not resolve to an email vendor.
func (s *UtilsTestSuite) TestEmailVendorNameUnregisteredProviderReturnsFalse() {
	name, ok := emailVendorName(ncommon.NotificationProviderTypeTwilio)
	s.False(ok)
	s.Empty(name)
}

func (s *UtilsTestSuite) TestIDPVendorName() {
	name, ok := idpVendorName(providers.IDPTypeGoogle)
	s.True(ok)
	s.Equal("google", name)
}

func (s *UtilsTestSuite) TestIDPVendorNameUnregisteredTypeReturnsFalse() {
	name, ok := idpVendorName(providers.IDPType("unregistered-type"))
	s.False(ok)
	s.Empty(name)
}

func (s *UtilsTestSuite) TestIsIDPBackedVendorName() {
	for _, name := range []string{"google", "github", "oidc", "oauth"} {
		s.True(isIDPBackedVendorName(name), name)
	}
	// Sender-backed and AuthZEN vendors are registered, but not IdP-backed.
	for _, name := range []string{"twilio", emailSMTPVendorName, authZENPDPVendorName, ""} {
		s.False(isIDPBackedVendorName(name), name)
	}
}
