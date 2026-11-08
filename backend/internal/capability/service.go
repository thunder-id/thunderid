// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package capability reports which management actions the caller may perform, so a client can decide
// what to offer. It decides nothing: every management API still authorizes its own requests.
package capability

import (
	"context"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// capabilityService answers each catalog action through the server's system authorization service,
// with the checks that don't depend on a record.
type capabilityService struct {
	authzService sysauthz.SystemAuthorizationServiceInterface
}

// newCapabilityService returns a capabilityService.
func newCapabilityService(authzService sysauthz.SystemAuthorizationServiceInterface) *capabilityService {
	return &capabilityService{authzService: authzService}
}

// GetCapabilities returns the management actions the caller may perform, and where.
func (s *capabilityService) GetCapabilities(ctx context.Context) (*capabilitiesResponse, *tidcommon.ServiceError) {
	allowed := make(map[security.Action]bool)
	resources := make(map[security.ResourceType]map[string]bool, len(requiredActions))
	for resource, catalogActions := range requiredActions {
		resources[resource] = make(map[string]bool, len(catalogActions))
		for name, actions := range catalogActions {
			if name != viewAction {
				actions = slices.Concat(catalogActions[viewAction], actions)
			}
			can, svcErr := s.allowsAll(ctx, actions, allowed)
			if svcErr != nil {
				return nil, svcErr
			}
			resources[resource][name] = can
		}
	}

	boundary := s.authzService.GetBoundary(ctx)
	return &capabilitiesResponse{
		Scope:     capabilityScope{Type: boundary.Type, OUID: boundary.OUID},
		Resources: resources,
		Root:      rootCapabilities{Manage: security.HasSystemPermission(security.GetPermissions(ctx))},
	}, nil
}

// allowsAll reports whether the caller may perform every one of actions, reusing and filling allowed so
// each action is evaluated once per request.
func (s *capabilityService) allowsAll(ctx context.Context, actions []security.Action,
	allowed map[security.Action]bool) (bool, *tidcommon.ServiceError) {
	for _, action := range actions {
		isAllowed, ok := allowed[action]
		if !ok {
			var svcErr *tidcommon.ServiceError
			isAllowed, svcErr = s.authzService.IsActionAllowed(ctx, action, nil)
			if svcErr != nil {
				return false, svcErr
			}
			allowed[action] = isAllowed
		}
		if !isAllowed {
			return false, nil
		}
	}
	return true, nil
}
