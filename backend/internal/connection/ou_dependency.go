// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ouDependencyProvider reports the connections an organization unit owns, which block the organization unit's
// deletion (a connection cannot exist without its owner).
type ouDependencyProvider struct {
	idpService          idp.IDPServiceInterface
	notificationService notification.NotificationSenderMgtSvcInterface
	authZENPDPService   authzenpdp.AuthZENPDPServiceInterface
	httpService         httpconnection.HTTPConnectionServiceInterface
}

// newOUDependencyProvider creates the dependency provider that reports the connections of every kind that belong to
// an organization unit.
func newOUDependencyProvider(
	idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface,
) resourcedependency.Provider {
	return &ouDependencyProvider{
		idpService:          idpService,
		notificationService: notificationService,
		authZENPDPService:   authZENPDPService,
		httpService:         httpService,
	}
}

// GetResourceDependencies implements resourcedependency.Provider. Only organization unit targets are handled; other
// resource types have no connection dependencies. The connections are listed with the runtime context, since whether
// an organization unit can be deleted does not depend on which connections the caller may see.
func (p *ouDependencyProvider) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	deps := make([]resourcedependency.ResourceDependency, 0)
	if resourceType != resourcedependency.ResourceTypeOU {
		return deps, nil
	}
	ctx = security.WithRuntimeContext(ctx)
	add := func(kind, connectionID, name string) {
		deps = append(deps, resourcedependency.ResourceDependency{
			ResourceType:     kind,
			ID:               connectionID,
			DisplayName:      name,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}

	idps, svcErr := p.idpService.GetIdentityProviderList(ctx)
	if svcErr != nil {
		return nil, serviceErrorToError(svcErr)
	}
	for _, instance := range idps {
		if instance.OUID == id {
			add(resourcedependency.ResourceTypeIDP, instance.ID, instance.Name)
		}
	}

	senders, svcErr := p.notificationService.ListSenders(ctx)
	if svcErr != nil {
		return nil, serviceErrorToError(svcErr)
	}
	for _, sender := range senders {
		if sender.OUID == id {
			add(resourcedependency.ResourceTypeNotificationSender, sender.ID, sender.Name)
		}
	}

	pdpConnections, svcErr := p.authZENPDPService.ListAuthZENPDPs(ctx)
	if svcErr != nil {
		return nil, serviceErrorToError(svcErr)
	}
	for _, connection := range pdpConnections {
		if connection.OUID == id {
			add(resourcedependency.ResourceTypeAuthZENPDPConnection, connection.ID, connection.Name)
		}
	}

	httpConnections, svcErr := p.httpService.ListHTTPConnections(ctx)
	if svcErr != nil {
		return nil, serviceErrorToError(svcErr)
	}
	for _, connection := range httpConnections {
		if connection.OUID == id {
			add(resourcedependency.ResourceTypeHTTPConnection, connection.ID, connection.Name)
		}
	}
	return deps, nil
}

// serviceErrorToError reports a service error as the error the dependency registry expects of a provider.
func serviceErrorToError(svcErr *tidcommon.ServiceError) error {
	return fmt.Errorf("%s: %s", svcErr.Error.DefaultValue, svcErr.ErrorDescription.DefaultValue)
}
