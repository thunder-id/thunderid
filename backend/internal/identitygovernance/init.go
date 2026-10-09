// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"

	governanceconfig "github.com/thunder-id/thunderid/internal/identitygovernance/config"
	governancenotification "github.com/thunder-id/thunderid/internal/identitygovernance/notification"

	sysconfig "github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// NotificationDependencies supplies the profile read, template and email sender services.
// Omitting these dependencies disables notices without disabling account locks.
type NotificationDependencies struct {
	// Profiles reads the recipient address.
	Profiles governancenotification.ProfileReader
	// Templates renders the account locked notice.
	Templates governancenotification.TemplateRenderer
	// Senders sends the notice.
	Senders governancenotification.EmailSender
	// DefaultSender names the email sender a notice goes through.
	DefaultSender governancenotification.SenderSource
}

// Initialize builds the identity governance service. entityState is required; notifications and
// observability are optional. accountAccess is the deployment lockout policy, the base of the
// accountAccess server-config section.
func Initialize(entityState EntityStateProvider, accountAccess sysconfig.AccountAccessConfig,
	notifications *NotificationDependencies,
	authorizer sysauthz.SystemAuthorizationServiceInterface, observability providers.ObservabilityProvider,
) (IdentityGovernanceServiceInterface, error) {
	if entityState == nil {
		return nil, ErrNoEntityStateProvider
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdentityGovernanceService"))
	section := governanceconfig.NewAccountAccessSource(accountAccess, logger)
	resolver := governanceconfig.NewPolicyResolver(section)
	notifier := initNotifier(section, notifications, logger)

	svc := newService(entityState, resolver, notifier, authorizer)
	svc.observability = observability
	return svc, nil
}

// initNotifier builds the notifier when its dependencies are available. It is built even when the
// email is off or no default sender is set, since both are read per notice.
func initNotifier(section *governanceconfig.AccountAccessSource, notifications *NotificationDependencies,
	logger *log.Logger) LockNotifier {
	if notifications == nil || notifications.Profiles == nil || notifications.Templates == nil ||
		notifications.Senders == nil || notifications.DefaultSender == nil {
		logger.Debug(context.Background(),
			"Account lock notification dependencies are not wired; no lock notification will be sent")
		return nil
	}
	return governancenotification.NewLockEmailNotifier(section, notifications.Profiles,
		notifications.Templates, notifications.Senders, notifications.DefaultSender)
}
