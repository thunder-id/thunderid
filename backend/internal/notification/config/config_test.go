// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

type NotificationConfigTestSuite struct {
	suite.Suite
}

func TestNotificationConfigTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationConfigTestSuite))
}

func (s *NotificationConfigTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (s *NotificationConfigTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *NotificationConfigTestSuite) TestFromServerRuntime() {
	for _, declarative := range []bool{true, false} {
		s.Run(fmt.Sprintf("declarative=%t", declarative), func() {
			config.ResetServerRuntime()
			cfg := &config.Config{
				Notification: config.NotificationConfig{OTP: config.OTPConfig{
					Length: 8, UseNumericOnly: new(false), ValidityPeriodSeconds: 300,
				}},
				JWT: engineconfig.JWTConfig{
					Issuer: "https://issuer.example.test", ValidityPeriod: 600,
				},
				DeclarativeResources: config.DeclarativeResources{Enabled: declarative},
			}
			s.Require().NoError(config.InitializeServerRuntime("/tmp/test-notification-config", cfg))

			result := FromServerRuntime()

			s.Equal(cfg.Notification.OTP, result.OTP)
			s.Equal(cfg.JWT, result.JWT)
			s.Equal(declarative, result.DeclarativeResourcesEnabled)
		})
	}
}
