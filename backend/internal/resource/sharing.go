// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/sharing"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// ResourceServerSharingType is the sharing framework's identifier for resource servers.
const ResourceServerSharingType sharing.ResourceType = resourceTypeResourceServer

// SharingFieldPermissions is the one field a resource server sharing policy governs: which of the
// server's permissions the reached organization units may use.
const SharingFieldPermissions = "permissions"

// resourceServerSharingDeclaration is how a resource server participates in sharing: the framework
// asks it what resource servers are called, which fields a policy may govern, and who owns one.
//
// Sharing a resource server makes its permissions available in the reached organization units, so
// they can be put on roles there and granted to tokens issued for them. It does not place the
// resource server in their listings or make it editable there.
type resourceServerSharingDeclaration struct {
	service ResourceServiceInterface
}

// newResourceServerSharingDeclaration returns the declaration registered with the sharing framework.
func newResourceServerSharingDeclaration(service ResourceServiceInterface) *resourceServerSharingDeclaration {
	return &resourceServerSharingDeclaration{service: service}
}

// ResourceType identifies resource servers to the sharing framework.
func (d *resourceServerSharingDeclaration) ResourceType() sharing.ResourceType {
	return ResourceServerSharingType
}

// Fields declares the permissions a policy shares, as a hierarchy over the server's permission
// strings: naming a resource shares it together with every resource and action beneath it.
//
// The default shares the whole server, so a policy that names no rule hands over every permission.
// It is pinned, because the sharing organization unit decides what is shared and a reached unit has
// nothing of its own to choose.
func (d *resourceServerSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return []sharing.FieldDeclaration{
		{
			Key:     SharingFieldPermissions,
			Kind:    sharing.FieldHierarchy,
			Default: &sharing.OverlayRule{Editable: false},
		},
	}
}

// OwningOUID returns the organization unit that owns a resource server.
func (d *resourceServerSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	server, svcErr := d.service.GetResourceServer(ctx, resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return server.OUID, nil
}

// FieldDelimiter returns the resource server's own delimiter, which is what joins the segments of
// its permission strings.
func (d *resourceServerSharingDeclaration) FieldDelimiter(
	ctx context.Context, resourceID, _ string,
) (string, *tidcommon.ServiceError) {
	server, svcErr := d.service.GetResourceServer(ctx, resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return server.Delimiter, nil
}

// ValidateOverlayRule refuses the rule shapes a resource server gives no meaning to.
//
// A rule names what is shared in value or what is withheld in excludedValues. allowedValues and
// editable describe a choice the reached organization unit makes for itself, and there is none to
// make here. Naming both value and excludedValues is refused as well: an exclusion inside a list
// that already names everything shared is a second way of writing a shorter list.
func (d *resourceServerSharingDeclaration) ValidateOverlayRule(
	_ context.Context, _, _ string, rule sharing.OverlayRule,
) *tidcommon.ServiceError {
	switch {
	case rule.AllowedValues != nil:
		return &ErrorSharingAllowedValuesNotSupported
	case rule.Editable:
		return &ErrorSharingEditableNotSupported
	case rule.Value != nil && rule.ExcludedValues != nil:
		return &ErrorSharingValueWithExcludedValues
	default:
		return nil
	}
}

// ValidateMembers refuses permissions the resource server does not define, or that the initiating
// organization unit does not itself hold. The second is what stops a reshare naming permissions its
// own sharer withheld, and is asked by validating as the initiating organization unit.
func (d *resourceServerSharingDeclaration) ValidateMembers(
	ctx context.Context, resourceID, _, initiatingOUID string, members []string,
) *tidcommon.ServiceError {
	invalid, svcErr := d.service.ValidatePermissions(
		syscontext.WithAccessingOUID(ctx, initiatingOUID), resourceID, members)
	if svcErr != nil {
		return svcErr
	}
	if len(invalid) > 0 {
		return &ErrorSharingPermissionsNotAvailable
	}
	return nil
}

// unavailablePermissions returns the permissions that defined on the server but not granted to the organization unit.
//
// A server no policy names is open to every organization unit, as it was before sharing existed.
// This is temporary, until resource servers can be shared through the API and the console.
func (rs *resourceService) unavailablePermissions(
	ctx context.Context, server providers.ResourceServer, ouID string, permissions []string,
) ([]string, *tidcommon.ServiceError) {
	if ouID == server.OUID || len(permissions) == 0 {
		return []string{}, nil
	}
	// Without the framework no policy can name the server, so it is open like any unshared one.
	if rs.sharingService == nil {
		return []string{}, nil
	}

	overlay, svcErr := rs.sharingService.ResolveOverlayRules(ctx, ResourceServerSharingType, server.ID, ouID)
	if svcErr != nil {
		rs.logger.Error(ctx, "Failed to resolve the permissions shared with an organization unit",
			log.String("resourceServerId", server.ID), log.String("ouID", ouID),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
	if !overlay.Visible {
		// Asked only here, so a unit the server is shared with never pays for the listing.
		policies, svcErr := rs.sharingService.ListPolicies(ctx, ResourceServerSharingType, server.ID)
		if svcErr != nil {
			rs.logger.Error(ctx, "Failed to list the sharing policies of a resource server",
				log.String("resourceServerId", server.ID), log.String("error", svcErr.Error.DefaultValue))
			return nil, &tidcommon.InternalServerError
		}
		if len(policies) == 0 {
			return []string{}, nil
		}
		return permissions, nil
	}

	// Every declared field is answered, by a policy or by the default, so the rule is always there
	// for a unit that can see the server.
	rule := overlay.Rules[SharingFieldPermissions]
	unavailable := make([]string, 0)
	for _, p := range permissions {
		if !sharing.RuleGrants(rule, sharing.FieldHierarchy, server.Delimiter, p) {
			unavailable = append(unavailable, p)
		}
	}
	return unavailable, nil
}
