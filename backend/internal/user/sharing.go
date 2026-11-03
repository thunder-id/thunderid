// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"fmt"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// UserSharingType is the sharing framework's identifier for users.
const UserSharingType sharing.ResourceType = resourceTypeUser

// RolesFieldKey is the sharing field naming the roles a user share mandates in a child unit it names.
const RolesFieldKey = "roles"

// userSharingDeclaration is how a user participates in sharing.
//
// Sharing a user makes the same identity visible in the organization units a policy reaches,
// without copying it. The user keeps living in its own organization unit.
type userSharingDeclaration struct {
	service        UserServiceInterface
	sharingService sharing.SharingServiceInterface
	// roleAssignments writes the roles a share mandates. It is injected once roles are built, which is after users.
	roleAssignments role.RoleAssignmentServiceInterface
	// pendingDeclared holds the declared policies loaded before roleAssignments, whose mandates wait for it.
	pendingDeclared []sharing.Policy
}

// newUserSharingDeclaration returns the declaration registered with the sharing framework.
func newUserSharingDeclaration(
	service UserServiceInterface, sharingService sharing.SharingServiceInterface,
) *userSharingDeclaration {
	return &userSharingDeclaration{service: service, sharingService: sharingService}
}

// ResourceType identifies users to the sharing framework.
func (d *userSharingDeclaration) ResourceType() sharing.ResourceType {
	return UserSharingType
}

// Fields declares the roles a share mandates for the user in each child unit it names.
func (d *userSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return []sharing.FieldDeclaration{{Key: RolesFieldKey, Kind: sharing.FieldReferenceSet}}
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

// ValidateMembers refuses a role the issuing unit cannot see; before roles are injected OnPolicyWritten decides.
func (d *userSharingDeclaration) ValidateMembers(
	ctx context.Context, _, fieldKey, initiatingOUID string, members []string,
) *tidcommon.ServiceError {
	if fieldKey != RolesFieldKey || d.roleAssignments == nil {
		return nil
	}
	return d.requireRolesVisible(ctx, initiatingOUID, members)
}

// OnPolicyWritten refuses a roles rule other than a fixed one on a named child unit, then applies the mandates.
func (d *userSharingDeclaration) OnPolicyWritten(ctx context.Context, p sharing.Policy) *tidcommon.ServiceError {
	if svcErr := requireFixedRolesOnNamedUnits(p); svcErr != nil {
		return svcErr
	}
	if d.roleAssignments == nil {
		if p.Declared {
			d.pendingDeclared = append(d.pendingDeclared, p)
			return nil
		}
		if len(mandatedRoles(p)) > 0 {
			log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).Error(ctx,
				"No role assignment service to apply the roles a sharing policy mandates", log.String("policyID", p.ID))
			return &tidcommon.InternalServerError
		}
		return nil
	}
	return d.applyMandatedRoles(ctx, p)
}

// OnPolicyRemoved removes the assignments the policy mandated.
func (d *userSharingDeclaration) OnPolicyRemoved(ctx context.Context, p sharing.Policy) *tidcommon.ServiceError {
	if d.roleAssignments == nil {
		return nil
	}
	return d.roleAssignments.RemoveMandatedAssignments(ctx, p.ID)
}

// setRoleAssignments injects the role assignment service, then checks and applies the declared mandates waiting.
func (d *userSharingDeclaration) setRoleAssignments(
	ctx context.Context, roleAssignments role.RoleAssignmentServiceInterface,
) error {
	d.roleAssignments = roleAssignments
	pending := d.pendingDeclared
	d.pendingDeclared = nil

	// Declaring carries no caller to scope against.
	ctx = security.WithRuntimeContext(ctx)
	for _, p := range pending {
		var named []string
		for _, r := range p.Rules {
			if r.FieldKey == RolesFieldKey && r.Requested.Value != nil {
				named = append(named, *r.Requested.Value...)
			}
		}
		svcErr := d.requireRolesVisible(ctx, p.InitiatingOUID, utils.UniqueStrings(named))
		if svcErr == nil {
			svcErr = d.applyMandatedRoles(ctx, p)
		}
		if svcErr != nil {
			return fmt.Errorf("failed to apply the roles declared sharing policy %q of user %q mandates: %s: %s",
				p.ID, p.ResourceID, svcErr.Code, svcErr.ErrorDescription.DefaultValue)
		}
	}
	return nil
}

// requireRolesVisible refuses a role the organization unit neither owns nor is shared.
func (d *userSharingDeclaration) requireRolesVisible(
	ctx context.Context, ouID string, roleIDs []string,
) *tidcommon.ServiceError {
	for _, roleID := range roleIDs {
		visible, svcErr := d.sharingService.IsVisible(ctx, role.RoleSharingType, roleID, ouID)
		if svcErr != nil {
			return svcErr
		}
		if !visible {
			return &role.ErrorMandatedRoleNotInOU
		}
	}
	return nil
}

// applyMandatedRoles makes the user's assignments the policy mandates match the roles its named units' rules pin.
func (d *userSharingDeclaration) applyMandatedRoles(ctx context.Context, p sharing.Policy) *tidcommon.ServiceError {
	return d.roleAssignments.ApplyMandatedAssignments(ctx, p.ID,
		role.RoleAssignment{ID: p.ResourceID, Type: role.AssigneeTypeUser}, mandatedRoles(p))
}

// requireFixedRolesOnNamedUnits refuses a roles rule off a named child unit of reach unit, or one not listing roles.
// The framework already refuses an editable roles rule and one with allowedValues.
func requireFixedRolesOnNamedUnits(p sharing.Policy) *tidcommon.ServiceError {
	for _, r := range p.Rules {
		if r.FieldKey != RolesFieldKey {
			continue
		}
		if r.Grant.Section != sharing.SectionChildUnits || !reachesNamedUnitOnly(p, r.Grant.OUID) {
			return &ErrorRolesNotOnNamedUnit
		}
		requested := r.Requested
		if requested.Value == nil || len(*requested.Value) == 0 || requested.ExcludedValues != nil {
			return &ErrorRolesRuleNotFixed
		}
	}
	return nil
}

// reachesNamedUnitOnly reports whether the policy names ouID as a child unit and reaches no further than it.
func reachesNamedUnitOnly(p sharing.Policy, ouID string) bool {
	if ouID == "" || p.ChildUnits == nil {
		return false
	}
	return slices.ContainsFunc(p.ChildUnits.Units, func(u sharing.ChildUnit) bool {
		return u.OUID == ouID && u.Reach == sharing.ReachUnit
	})
}

// mandatedRoles maps each child unit the policy names to the roles its rule pins there.
func mandatedRoles(p sharing.Policy) map[string][]string {
	desired := make(map[string][]string)
	if p.ChildUnits == nil || p.ChildUnits.Mode != sharing.ChildUnitsSelected {
		return desired
	}
	for _, u := range p.ChildUnits.Units {
		rule, ok := p.GrantRules(sharing.GrantRef{Section: sharing.SectionChildUnits, OUID: u.OUID})[RolesFieldKey]
		if ok && rule.Value != nil && len(*rule.Value) > 0 {
			desired[u.OUID] = slices.Clone(*rule.Value)
		}
	}
	return desired
}
