// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

const (
	// ResourceServerSharingType is the sharing framework's identifier for resource servers.
	ResourceServerSharingType sharing.ResourceType = "resource_server"

	// ResourcesFieldKey is the one field a resource server exposes to sharing policies. Its members
	// are permission strings, which is what makes the authorization path compare like with like.
	ResourcesFieldKey = "resources"
)

// resourceServerSharing declares how a resource server participates in sharing.
//
// One field, and it is hierarchical rather than a set of node ids: a permission string already
// denotes itself and everything beneath it, so "share the booking tree except refunds" is one rule
// with one exclusion instead of a grant per node. Newly added descendants are covered without a
// replay, and the resolved members are the same strings a token request carries.
type resourceServerSharing struct {
	service ResourceServiceInterface
}

// newResourceServerSharing returns the declaration registered with the sharing framework.
func newResourceServerSharing(service ResourceServiceInterface) *resourceServerSharing {
	return &resourceServerSharing{service: service}
}

// ResourceType identifies resource servers to the sharing framework.
func (d *resourceServerSharing) ResourceType() sharing.ResourceType {
	return ResourceServerSharingType
}

// Fields declares the resources field.
//
// The default is not editable with no value, which resolves to the owner's whole permission tree
// minus any exclusions: sharing a server with no overlay rule at all hands over everything, which
// is what a cascade share with no withheld nodes used to mean.
func (d *resourceServerSharing) Fields() []sharing.FieldDeclaration {
	inherited := sharing.OverlayRule{Editable: false}
	return []sharing.FieldDeclaration{
		{
			Key:     ResourcesFieldKey,
			Kind:    sharing.FieldHierarchy,
			Default: &inherited,
		},
	}
}

// OwningOUID returns the organization unit that owns a resource server.
func (d *resourceServerSharing) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	// Runtime context: the framework is resolving ownership in order to decide access, so this
	// read must not itself be access-checked. It would otherwise recurse through IsVisible.
	server, svcErr := d.service.GetResourceServer(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return server.OUID, nil
}

// FieldDelimiter returns the server's own permission separator.
//
// Servers do not agree on one: the separator is per server, so comparing paths with any other
// character would read a sibling as a descendant, or fail to recognize a real descendant at all.
func (d *resourceServerSharing) FieldDelimiter(
	ctx context.Context, resourceID, _ string,
) (string, *tidcommon.ServiceError) {
	// Runtime context, for the same reason as OwningOUID above.
	server, svcErr := d.service.GetResourceServer(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return server.Delimiter, nil
}

// ValidateMembers rejects a rule naming a permission the server does not define.
//
// Without this an initiator could pin or allow a path that exists nowhere, which would read as a
// silently empty grant later rather than as the authoring mistake it is.
func (d *resourceServerSharing) ValidateMembers(
	ctx context.Context, resourceID, fieldKey, _ string, members []string,
) error {
	if fieldKey != ResourcesFieldKey || len(members) == 0 {
		return nil
	}

	// No organization unit: the question here is only whether the server defines these paths. Naming
	// one would ask the framework what that unit can see, from inside the framework's own validation
	// of the policy that decides it.
	invalid, svcErr := d.service.ValidatePermissions(
		security.WithRuntimeContext(ctx), resourceID, members, "")
	if svcErr != nil {
		return fmt.Errorf("failed to validate permissions: %s", svcErr.Code)
	}
	if len(invalid) > 0 {
		return fmt.Errorf("resource server defines no permission %q", invalid[0])
	}
	return nil
}

// ApplySharingPolicies records the sharing policies an imported resource server carries.
//
// Imported policies are stored rather than held in memory. The declarative loader re-seeds a file's
// policies on every start, so those can live in memory; an import runs once and leaves the resource
// in the database, with no file to re-read or revert to.
//
// A policy that already exists for an organization unit is left alone. Re-importing a bundle should
// not undo a narrowing an operator applied afterwards, and the import is idempotent as a result.
func (rs *resourceService) ApplySharingPolicies(
	ctx context.Context, serverID string, policies []providers.SharingPolicy,
) *tidcommon.ServiceError {
	if rs.sharingService == nil || len(policies) == 0 {
		return nil
	}

	// An import carries no caller to scope against.
	ctx = security.WithRuntimeContext(ctx)

	server, svcErr := rs.GetResourceServer(ctx, serverID)
	if svcErr != nil {
		return svcErr
	}

	for _, declared := range policies {
		_, svcErr := rs.sharingService.CreatePolicy(
			ctx, ResourceServerSharingType, serverID, server.OUID, sharing.RequestFromDeclaration(declared))
		if svcErr == nil {
			continue
		}
		if svcErr.Code == sharing.ErrorPolicyExists.Code {
			rs.logger.Debug(ctx, "Sharing policy already recorded for this organization unit",
				log.String("resourceServerId", serverID),
				log.String("initiatingOuId", declared.InitiatingOuID))
			continue
		}
		return mapSharingError(svcErr)
	}
	return nil
}

// appendPermissionsHiddenFromOU is the organization-unit half of ValidatePermissions: it adds to
// invalid every permission the resource server defines that ouID cannot see.
//
// It answers on that organization unit's behalf rather than the caller's, which is what the token
// path needs: a client_credentials request carries an application as its subject, not a caller with
// standing over organization units.
func (rs *resourceService) appendPermissionsHiddenFromOU(
	ctx context.Context, server providers.ResourceServer, permissions, invalid []string, ouID string,
) ([]string, *tidcommon.ServiceError) {
	// The organization unit owning the resource server sees everything beneath it.
	if ouID == "" || server.OUID == ouID {
		return invalid, nil
	}

	// Already-invalid permissions are skipped rather than reported twice.
	seen := make(map[string]struct{}, len(invalid))
	for _, p := range invalid {
		seen[p] = struct{}{}
	}

	// Nothing reached this organization unit, so every permission the server defines is hidden from
	// it. Resolving the tree filter first would reach the same answer one lookup later.
	if rs.sharingService == nil {
		return appendUnseen(invalid, seen, permissions, nil), nil
	}
	visible, svcErr := rs.sharingService.IsVisible(ctx, ResourceServerSharingType, server.ID, ouID)
	if svcErr != nil {
		return nil, mapSharingError(svcErr)
	}
	if !visible {
		return appendUnseen(invalid, seen, permissions, nil), nil
	}

	filter, svcErr := rs.sharedTreeFilter(ctx, &server, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	return appendUnseen(invalid, seen, permissions, filter.permits), nil
}

// appendUnseen adds every permission that permits rejects, skipping those already reported. A nil
// permits rejects everything.
func appendUnseen(
	invalid []string, seen map[string]struct{}, permissions []string, permits func(string) bool,
) []string {
	// The deployment root permission is a bypass rather than a grant: no resource server defines it
	// and it belongs to no organization unit, so the tree filter has no opinion on it. Read through
	// the nil-safe accessor, because the direct one panics before system permissions are initialized
	// and a panic on the token path is a worse failure than not recognizing the bypass.
	var rootPermission string
	if sysPerms := security.GetSystemPermissions(); sysPerms != nil {
		rootPermission = sysPerms.Root
	}

	for _, permission := range permissions {
		if _, already := seen[permission]; already {
			continue
		}
		if rootPermission != "" && permission == rootPermission {
			continue
		}
		if permits != nil && permits(permission) {
			continue
		}
		invalid = append(invalid, permission)
		seen[permission] = struct{}{}
	}
	return invalid
}
