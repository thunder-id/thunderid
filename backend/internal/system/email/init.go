// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package email

import "github.com/thunder-id/thunderid/internal/system/config"

// Initialize creates and returns the configured email client. It verifies SMTP
// connectivity on startup so an unreachable mail server fails fast instead of only
// failing when the first email is sent.
func Initialize() (EmailClientInterface, error) {
	client, err := NewSMTPClientFromConfig()
	if err != nil {
		return nil, err
	}

	smtpConf := config.GetServerRuntime().Config.Email.SMTP
	if err := checkSMTPConnectivity(smtpConf.Host, smtpConf.Port); err != nil {
		return nil, err
	}

	return client, nil
}
