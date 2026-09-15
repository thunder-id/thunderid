// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// AdminProviderInterface is the resource-catalog surface the scope deletion flow acts through. It is
// separate from the role and group surface because deleting a scope is a change to the catalog, not to
// who holds what: it needs no role or group lookup, and it revokes deployment-wide.
type AdminProviderInterface interface {
	// ValidateDeleteAction reports whether the action may be deleted, and returns the scope that stops
	// existing when it is. It changes no state.
	//
	// An empty resourceID names an action defined directly on the resource server rather than under one
	// of its resources.
	ValidateDeleteAction(ctx context.Context, resourceServerID, resourceID, actionID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	// DeleteAction deletes the action, retiring the scope it defines.
	DeleteAction(ctx context.Context, resourceServerID, resourceID, actionID string) *tidcommon.ServiceError
}

// adminProvider serves the catalog operations an administration flow performs. It wraps the resource
// service rather than extending it, because a flow needs the scope an action defines and the audience
// that gives it meaning together, and the service returns them from two calls.
type adminProvider struct {
	resourceService ResourceServiceInterface
	logger          *log.Logger
}

var _ AdminProviderInterface = (*adminProvider)(nil)

// newAdminProvider creates the provider backing the scope deletion flow.
func newAdminProvider(resourceService ResourceServiceInterface) AdminProviderInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	return &adminProvider{resourceService: resourceService, logger: logger}
}

// ValidateDeleteAction reports whether the action may be deleted and returns the scope that stops
// existing when it is. It changes no state.
//
// The target carries no principals: a retired scope should be held by nobody, so the revocation is
// deployment-wide rather than enumerated per holder, which would miss whoever gets a token next.
func (p *adminProvider) ValidateDeleteAction(ctx context.Context, resourceServerID, resourceID,
	actionID string) (*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
	if resourceServerID == "" || actionID == "" {
		return nil, &ErrorMissingID
	}

	// A declarative resource server owns its catalog from a file, so the deletion would be refused.
	// Refusing here rather than at the acting node keeps the flow from retiring a scope that is about
	// to still exist.
	if p.resourceService.IsResourceServerDeclarative(resourceServerID) {
		return nil, ErrorImmutableAction.WithParams(map[string]string{"id": actionID})
	}

	resourceServer, svcErr := p.resourceService.GetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}
	action, svcErr := p.resourceService.GetAction(ctx, resourceServerID, optionalID(resourceID), actionID)
	if svcErr != nil {
		return nil, svcErr
	}

	// A resource server with no identifier issues no audience, so no token can carry this scope in a
	// form a criterion could match. Nothing to revoke, and the deletion still proceeds.
	if resourceServer == nil || resourceServer.Identifier == "" || action == nil || action.Permission == "" {
		return &revocation.ScopeRevocationTarget{}, nil
	}

	return &revocation.ScopeRevocationTarget{
		Scopes: []revocation.AudienceScope{
			{Audience: resourceServer.Identifier, Scope: action.Permission},
		},
	}, nil
}

// DeleteAction deletes the action, retiring the scope it defines.
func (p *adminProvider) DeleteAction(ctx context.Context, resourceServerID, resourceID,
	actionID string) *tidcommon.ServiceError {
	if svcErr := p.resourceService.DeleteAction(ctx, resourceServerID,
		optionalID(resourceID), actionID); svcErr != nil {
		p.logger.Error(ctx, "Failed to delete action",
			log.String("resourceServerId", resourceServerID), log.String("actionId", actionID),
			log.String("code", svcErr.Code), log.String("reason", svcErr.Error.DefaultValue))
		return svcErr
	}
	return nil
}

// optionalID converts an absent identifier to the nil the resource service reads as "not scoped to a
// resource". The service rejects a pointer to an empty string, so the two cannot be conflated.
func optionalID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
