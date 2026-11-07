// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// deploymentBoundedAuthzService authorizes a caller on any organization unit in the deployment. The
// organization unit is a resource here, not a boundary: the deployment's own partitioning keeps the
// caller inside it.
type deploymentBoundedAuthzService struct {
	*systemAuthorizationService
}

// newDeploymentBoundedAuthzService returns a deploymentBoundedAuthzService.
func newDeploymentBoundedAuthzService() *deploymentBoundedAuthzService {
	return &deploymentBoundedAuthzService{systemAuthorizationService: newSystemAuthorizationService()}
}

// IsActionAllowed allows a caller holding the action's permission on any organization unit in the
// deployment, including creating root organization units.
func (s *deploymentBoundedAuthzService) IsActionAllowed(ctx context.Context, action security.Action,
	actionCtx *ActionContext) (bool, *tidcommon.ServiceError) {
	if allowed, decided := s.authorizeCaller(ctx, action, actionCtx); decided {
		return allowed, nil
	}

	logger := s.logger.WithContext(ctx)
	if logger.IsDebugEnabled() {
		logger.Debug(ctx, "Authorization granted",
			log.String("action", string(action)),
			log.MaskedString("subject", security.GetSubject(ctx)))
	}
	return true, nil
}

// GetAccessibleResources returns every resource in the deployment to a caller holding the action's
// permission.
func (s *deploymentBoundedAuthzService) GetAccessibleResources(ctx context.Context, action security.Action,
	resourceType security.ResourceType) (*AccessibleResources, *tidcommon.ServiceError) {
	if result, decided := s.authorizeListCaller(ctx, action, resourceType); decided {
		return result, nil
	}
	return &AccessibleResources{AllAllowed: true}, nil
}

// SetOUHierarchyResolver ignores the resolver, since the deployment boundary never consults the
// organization unit hierarchy.
func (s *deploymentBoundedAuthzService) SetOUHierarchyResolver(OUHierarchyResolver) {}
