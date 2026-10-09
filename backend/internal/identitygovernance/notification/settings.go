// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"strings"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// recipientAttributeFor returns the profile attribute a lock notice for this category is sent to, and
// false when the category is not notified. Only users are notified. An unset or blank attribute uses the default.
func recipientAttributeFor(cfg governanceconfig.AccountAccessValue,
	category providers.EntityCategory) (string, bool) {
	if category != providers.EntityCategoryUser {
		return "", false
	}
	settings := userLockEmail(cfg)
	if settings.Enabled == nil || !*settings.Enabled {
		return "", false
	}
	attribute := DefaultRecipientAttribute
	if settings.RecipientAttribute != nil {
		if configured := strings.TrimSpace(*settings.RecipientAttribute); configured != "" {
			attribute = configured
		}
	}
	return attribute, true
}

// userLockEmail returns the user category's lock email block, or an empty block if unset.
func userLockEmail(cfg governanceconfig.AccountAccessValue) governanceconfig.AccountAccessLockEmail {
	if cfg.User == nil {
		return governanceconfig.AccountAccessLockEmail{}
	}
	if email := governanceconfig.LockEmailOf(cfg.User.Notifications); email != nil {
		return *email
	}
	return governanceconfig.AccountAccessLockEmail{}
}
