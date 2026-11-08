// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package capability reports which management actions the caller may perform, so a client can decide
// what to offer. It decides nothing: every management API still authorizes its own requests.
package capability

import (
	"context"

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
	resources := make(map[security.ResourceType]map[string]bool)
	for _, catalogAction := range security.CatalogActions() {
		allowed, svcErr := s.authzService.IsActionAllowed(ctx, catalogAction.Action, nil)
		if svcErr != nil {
			return nil, svcErr
		}
		if resources[catalogAction.Resource] == nil {
			resources[catalogAction.Resource] = make(map[string]bool)
		}
		resources[catalogAction.Resource][catalogAction.Name] = allowed
	}

	canViewRoot, svcErr := s.authzService.IsActionAllowed(ctx, security.ActionViewRootOnly, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	canManageRoot, svcErr := s.authzService.IsActionAllowed(ctx, security.ActionManageRootOnly, nil)
	if svcErr != nil {
		return nil, svcErr
	}

	boundary := s.authzService.GetBoundary(ctx)
	return &capabilitiesResponse{
		Scope:     capabilityScope{Type: boundary.Type, OUID: boundary.OUID},
		Resources: resources,
		Root:      rootCapabilities{View: canViewRoot, Manage: canManageRoot},
	}, nil
}
