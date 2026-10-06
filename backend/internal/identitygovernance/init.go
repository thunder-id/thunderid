// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the identity governance service and the accountAccess server-config handler. Only
// entityState is required. The server-config service is built with the handler, so it is set on the
// service afterwards with SetServerConfigService; until then cfg applies and the lock email has no
// sender. The returned function stops the lock email workers, waiting a bounded time for queued emails.
func Initialize(entityState entityStateProvider, cfg Config, notifications *NotificationDependencies,
	authorizer sysauthz.SystemAuthorizationServiceInterface, observability providers.ObservabilityProvider) (
	IdentityGovernanceServiceInterface, serverconfig.ServerConfigHandlerInterface, func(), error) {
	if entityState == nil {
		return nil, nil, nil, errNoEntityStateProvider
	}

	source := newAccountAccessSource(cfg.AccountAccess)
	stop := func() {}
	var notifier lockNotifierInterface
	if notifications != nil && notifications.Templates != nil && notifications.Senders != nil &&
		notifications.EntityTypes != nil {
		// Built even when the email is off or no default sender is set, since both are read per notice.
		notifier = newLockEmailNotifier(source, entityState, notifications.EntityTypes, notifications.Templates,
			notifications.Senders)
		stop = notifier.Close
	}

	service := newIdentityGovernanceService(entityState, source, notifier, authorizer, observability)
	return service, accountAccessHandler{base: cfg.AccountAccess}, stop, nil
}
