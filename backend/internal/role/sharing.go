// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// RoleSharingType is the sharing framework's identifier for roles.
const RoleSharingType sharing.ResourceType = resourceTypeRole

// roleAssignmentsFieldKey is the field governing whether an organization unit a role reaches may
// assign the role to its own members.
const roleAssignmentsFieldKey = "assignments"

// roleSharingDeclaration is how a role participates in sharing: the framework asks it what roles
// are called, which fields a policy may govern, and who owns one.
type roleSharingDeclaration struct {
	service           RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
	roleStore         roleStoreInterface
	ouService         oupkg.OrganizationUnitServiceInterface
}

// newRoleSharingDeclaration returns the declaration registered with the sharing framework.
func newRoleSharingDeclaration(
	service RoleServiceInterface, assignmentService RoleAssignmentServiceInterface, roleStore roleStoreInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
) *roleSharingDeclaration {
	return &roleSharingDeclaration{
		service: service, assignmentService: assignmentService, roleStore: roleStore, ouService: ouService,
	}
}

// ResourceType identifies roles to the sharing framework.
func (d *roleSharingDeclaration) ResourceType() sharing.ResourceType {
	return RoleSharingType
}

// Fields declares the role's assignments, which every organization unit the role reaches may make
// unless a policy turns them off. A policy only switches them on or off, naming no members. A
// role's name and permissions stay with its owner.
func (d *roleSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return []sharing.FieldDeclaration{{
		Key:          roleAssignmentsFieldKey,
		Kind:         sharing.FieldReferenceSet,
		EditableOnly: true,
		Default:      &sharing.OverlayRule{Editable: true},
	}}
}

// OwningOUID returns the organization unit that owns a role.
func (d *roleSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	role, svcErr := d.service.GetRoleWithPermissions(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return role.OUID, nil
}

// DeleteOverlayValues removes the assignments an organization unit made for a role once the role is
// no longer shared with it. Assignments live in the role's own table rather than the framework's.
// It refuses while a user share mandates the role in a unit that still exists.
func (d *roleSharingDeclaration) DeleteOverlayValues(ctx context.Context, resourceID, ouID string) error {
	mandated, err := d.roleStore.ListMandatedAssignmentsForRole(ctx, resourceID, ouID)
	if err != nil {
		return fmt.Errorf("failed to list the mandated assignments of role %s: %w", resourceID, err)
	}
	if len(mandated) > 0 {
		exists, svcErr := d.ouService.IsOrganizationUnitExists(ctx, ouID)
		if svcErr != nil {
			return fmt.Errorf("failed to check organization unit %s exists: code=%s", ouID, svcErr.Code)
		}
		if exists {
			return sharing.NewResourceTypeError(&ErrorRoleMandatedInOU)
		}
	}
	return d.assignmentService.DeleteAssignmentsInOU(ctx, resourceID, ouID)
}
