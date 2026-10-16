// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize builds the identity governance service. Only entityState is required; serverConfig
// supplies the accountAccess section, else cfg applies. The returned function drains queued lock
// emails on shutdown.
func Initialize(entityState EntityStateProvider, cfg Config, serverConfig serverconfig.ServerConfigService,
	notifications *NotificationDependencies, authorizer sysauthz.SystemAuthorizationServiceInterface,
	observability providers.ObservabilityProvider) (IdentityGovernanceServiceInterface, func(), error) {
	if entityState == nil {
		return nil, nil, errNoEntityStateProvider
	}

	source := newAccountAccessSource(cfg.AccountAccess, serverConfig)
	stop := func() {}
	var notifier lockNotifierInterface
	if notifications != nil && notifications.Templates != nil && notifications.Senders != nil &&
		notifications.DefaultSender != nil {
		// Built even when the email is off or no default sender is set, since both are read per notice.
		notifier = newLockEmailNotifier(source, entityState, notifications.Templates, notifications.Senders,
			notifications.DefaultSender)
		stop = notifier.Close
	} else {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceService")).Debug(
			context.Background(),
			"Account lock notification dependencies are not wired; no lock notification will be sent")
	}

	return newService(entityState, source, notifier, authorizer, observability), stop, nil
}
