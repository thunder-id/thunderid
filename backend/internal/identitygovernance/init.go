// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the identity governance service. entityState and serverConfig are required: the
// accountAccess section and the lock email's default sender are read through serverConfig. The returned
// notifier is nil when none is built; the caller stops it at shutdown.
func Initialize(entityState entityStateProvider, serverConfig serverconfig.ServerConfigService, cfg Config,
	notifications *NotificationDependencies, authorizer sysauthz.SystemAuthorizationServiceInterface,
	observability providers.ObservabilityProvider) (IdentityGovernanceServiceInterface, LockNotifierInterface, error) {
	if entityState == nil {
		return nil, nil, errNoEntityStateProvider
	}
	if serverConfig == nil {
		return nil, nil, errNoServerConfigService
	}

	source := newAccountAccessSource(cfg.AccountAccess, serverConfig)
	var notifier LockNotifierInterface
	if notifications != nil && notifications.Templates != nil && notifications.Senders != nil &&
		notifications.EntityTypes != nil {
		// Built even when the email is off or no default sender is set, since both are read per notice.
		notifier = newLockEmailNotifier(source, serverConfig, entityState, notifications.EntityTypes,
			notifications.Templates, notifications.Senders)
	}

	service := newIdentityGovernanceService(entityState, source, notifier, authorizer, observability)
	return service, notifier, nil
}
