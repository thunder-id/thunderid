// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// UserSharingType is the sharing framework's identifier for users.
const UserSharingType sharing.ResourceType = resourceTypeUser

// userSharingDeclaration is how a user participates in sharing.
//
// Sharing a user makes the same identity visible in the organization units a policy reaches,
// without copying it. The user keeps living in its own organization unit.
type userSharingDeclaration struct {
	service UserServiceInterface
}

// newUserSharingDeclaration returns the declaration registered with the sharing framework.
func newUserSharingDeclaration(service UserServiceInterface) *userSharingDeclaration {
	return &userSharingDeclaration{service: service}
}

// ResourceType identifies users to the sharing framework.
func (d *userSharingDeclaration) ResourceType() sharing.ResourceType {
	return UserSharingType
}

// Fields declares nothing yet, so a policy governs reach only and no organization unit may override
// any of the user's fields. The user's templated fields, the attributes a unit may set for itself,
// come later.
func (d *userSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return nil
}

// OwningOUID returns the organization unit a user lives in.
func (d *userSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	user, svcErr := d.service.GetUser(security.WithRuntimeContext(ctx), resourceID, false)
	if svcErr != nil {
		return "", svcErr
	}
	return user.OUID, nil
}

// userPageInOU returns one page of the users an organization unit owns, followed by the ones shared
// into it. Filters narrow the owned users only. An organization unit that does not exist has none.
func userPageInOU(
	ctx context.Context, entityService entity.EntityServiceInterface, sharingService sharing.SharingServiceInterface,
	ouID string, limit, offset int, filters map[string]interface{},
) (*entity.OUEntityPage, error) {
	sharedIDs, svcErr := sharingService.ListVisibleResourceIDs(ctx, UserSharingType, ouID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return &entity.OUEntityPage{Owned: []providers.Entity{}, Shared: []providers.Entity{}}, nil
		}
		return nil, fmt.Errorf("failed to list users shared into organization unit: %s", svcErr.Error.DefaultValue)
	}
	return entityService.GetEntityPageInOU(
		ctx, providers.EntityCategoryUser, ouID, sharedIDs, limit, offset, filters)
}
