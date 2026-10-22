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
// function stops the lock email workers, waiting a bounded time for queued emails.
func Initialize(entityState entityStateProvider, serverConfig serverconfig.ServerConfigService, cfg Config,
	notifications *NotificationDependencies, authorizer sysauthz.SystemAuthorizationServiceInterface,
	observability providers.ObservabilityProvider) (IdentityGovernanceServiceInterface, func(), error) {
	if entityState == nil {
		return nil, nil, errNoEntityStateProvider
	}
	if serverConfig == nil {
		return nil, nil, errNoServerConfigService
	}

	source := newAccountAccessSource(cfg.AccountAccess, serverConfig)
	stop := func() {}
	var notifier lockNotifierInterface
	if notifications != nil && notifications.Templates != nil && notifications.Senders != nil &&
		notifications.EntityTypes != nil {
		// Built even when the email is off or no default sender is set, since both are read per notice.
		notifier = newLockEmailNotifier(source, serverConfig, entityState, notifications.EntityTypes,
			notifications.Templates, notifications.Senders)
		stop = notifier.Close
	}

	service := newIdentityGovernanceService(entityState, source, notifier, authorizer, observability)
	return service, stop, nil
}
