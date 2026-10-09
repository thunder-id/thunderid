// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// AccountAccessNotificationsTestSuite tests how the notification block converts and layers.
type AccountAccessNotificationsTestSuite struct {
	suite.Suite
}

func TestAccountAccessNotificationsTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessNotificationsTestSuite))
}

// notificationValue builds a section stating only the user notification block.
func notificationValue(email *AccountAccessLockEmail) AccountAccessValue {
	return AccountAccessValue{User: &AccountAccessCategory{
		Notifications: &AccountAccessNotifications{OnLock: &AccountAccessLockNotification{Email: email}},
	}}
}

// The section carries the notification block to its readers.
func (s *AccountAccessNotificationsTestSuite) TestTheSectionCarriesTheNotificationBlock() {
	section := notificationValue(&AccountAccessLockEmail{
		Enabled:            boolPtr(true),
		RecipientAttribute: stringPtr("workEmail"),
	})

	email := LockEmailOf(section.User.Notifications)

	s.Require().NotNil(email)
	s.True(derefBool(email.Enabled, false))
	s.Equal("workEmail", derefString(email.RecipientAttribute))
}

// An omitted notification override keeps the declarative defaults.
func (s *AccountAccessNotificationsTestSuite) TestDeclarativeNotificationDefaultsSurviveAnOverride() {
	readOnly := notificationValue(&AccountAccessLockEmail{Enabled: boolPtr(true)})
	merged := AccountAccessHandler{}.Merge(readOnly,
		AccountAccessValue{User: &AccountAccessCategory{}}).(AccountAccessValue)
	s.True(derefBool(LockEmailOf(merged.User.Notifications).Enabled, false))
}

func stringPtr(v string) *string { return &v }
