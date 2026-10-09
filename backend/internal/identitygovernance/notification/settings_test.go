// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"testing"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type RecipientAttributeTestSuite struct {
	suite.Suite
}

func TestRecipientAttributeTestSuite(t *testing.T) {
	suite.Run(t, new(RecipientAttributeTestSuite))
}

func stringPtr(v string) *string { return &v }

func boolPtr(v bool) *bool { return &v }

func lockEmailConfig(email governanceconfig.AccountAccessLockEmail) governanceconfig.AccountAccessValue {
	return governanceconfig.AccountAccessValue{User: &governanceconfig.AccountAccessCategory{
		Notifications: &governanceconfig.AccountAccessNotifications{
			OnLock: &governanceconfig.AccountAccessLockNotification{Email: &email},
		},
	}}
}

// staticSource serves one fixed configuration.
type staticSource struct {
	cfg governanceconfig.AccountAccessValue
}

func (s staticSource) Current(context.Context) governanceconfig.AccountAccessValue {
	return s.cfg
}

func testUser() model.GovernedEntity {
	return model.GovernedEntity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "member"}
}

func (s *RecipientAttributeTestSuite) TestMasterSwitchDefaultsOff() {
	_, enabled := recipientAttributeFor(
		lockEmailConfig(governanceconfig.AccountAccessLockEmail{}), providers.EntityCategoryUser)
	s.False(enabled)
}

func (s *RecipientAttributeTestSuite) TestDefaultsToTheEmailAttribute() {
	attribute, enabled := recipientAttributeFor(lockEmailConfig(governanceconfig.AccountAccessLockEmail{
		Enabled: boolPtr(true),
	}), providers.EntityCategoryUser)
	s.True(enabled)
	s.Equal(DefaultRecipientAttribute, attribute)
}

func (s *RecipientAttributeTestSuite) TestReadsTheNamedAttribute() {
	attribute, enabled := recipientAttributeFor(lockEmailConfig(governanceconfig.AccountAccessLockEmail{
		Enabled:            boolPtr(true),
		RecipientAttribute: stringPtr("workEmail"),
	}), providers.EntityCategoryUser)
	s.True(enabled)
	s.Equal("workEmail", attribute)
}

// Agents and applications are not notified.
func (s *RecipientAttributeTestSuite) TestOnlyUsersAreNotified() {
	_, enabled := recipientAttributeFor(lockEmailConfig(governanceconfig.AccountAccessLockEmail{
		Enabled: boolPtr(true),
	}), providers.EntityCategoryAgent)
	s.False(enabled)
}

func (s *RecipientAttributeTestSuite) TestBlankAttributeUsesDefault() {
	for _, value := range []string{"", " ", "\t"} {
		attribute, enabled := recipientAttributeFor(lockEmailConfig(governanceconfig.AccountAccessLockEmail{
			Enabled: boolPtr(true), RecipientAttribute: stringPtr(value),
		}), providers.EntityCategoryUser)
		s.True(enabled)
		s.Equal(DefaultRecipientAttribute, attribute)
	}
}
