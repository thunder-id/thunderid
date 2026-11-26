// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// ResourceServerSharingType is the sharing framework's identifier for resource servers.
const ResourceServerSharingType sharing.ResourceType = resourceTypeResourceServer

// permissionsFieldKey is the sharing field holding the permissions of a resource server a policy shares.
const permissionsFieldKey = "permissions"

// resourceServerSharingDeclaration is how a resource server participates in sharing.
//
// Sharing a resource server makes it available, as a whole, in the organization units a policy reaches: a reached
// unit gets every resource, sub-resource and action the server defines. The server keeps living in its own
// organization unit, and only its owner changes its definition.
type resourceServerSharingDeclaration struct {
	service *resourceService
}

// The framework finds these capabilities by type assertion, so a signature that drifts would silently drop them.
var (
	_ sharing.FieldDelimiterResolver = (*resourceServerSharingDeclaration)(nil)
	_ sharing.PolicyWriteHooks       = (*resourceServerSharingDeclaration)(nil)
	_ sharing.OverlayCleaner         = (*resourceServerSharingDeclaration)(nil)
)

// newResourceServerSharingDeclaration returns the declaration registered with the sharing framework.
func newResourceServerSharingDeclaration(service *resourceService) *resourceServerSharingDeclaration {
	return &resourceServerSharingDeclaration{service: service}
}

// ResourceType identifies resource servers to the sharing framework.
func (d *resourceServerSharingDeclaration) ResourceType() sharing.ResourceType {
	return ResourceServerSharingType
}

// Fields declares the permissions a policy shares, compared as a hierarchy so naming a resource covers the
// resources and actions beneath it. A policy with no permissions rule shares every permission, fixed: a reached
// organization unit cannot choose its own, so the framework refuses an editable rule or allowedValues as wider
// than the owner holds. A policy cannot yet name the permissions it shares or withholds, which OnPolicyWritten
// refuses.
func (d *resourceServerSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return []sharing.FieldDeclaration{{
		Key:     permissionsFieldKey,
		Kind:    sharing.FieldHierarchy,
		Default: &sharing.OverlayRule{Editable: false},
	}}
}

// FieldDelimiter returns the delimiter the resource server joins its permissions with.
func (d *resourceServerSharingDeclaration) FieldDelimiter(
	ctx context.Context, resourceID, _ string,
) (string, *tidcommon.ServiceError) {
	resourceServer, svcErr := d.service.GetResourceServer(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return resourceServer.Delimiter, nil
}

// OnPolicyWritten refuses a permissions rule naming what it shares or withholds, on every path a policy is written
// by. A resource server keeps no state per organization unit, so the change needs no revocation.
func (d *resourceServerSharingDeclaration) OnPolicyWritten(
	_ context.Context, p sharing.Policy,
) ([]sharing.Revocation, *tidcommon.ServiceError) {
	for _, r := range p.Rules {
		if r.FieldKey == permissionsFieldKey && (r.Requested.Value != nil || r.Requested.ExcludedValues != nil) {
			return nil, &ErrorPermissionsRuleNotSupported
		}
	}
	return nil, nil
}

// DeleteOverlayValues is called for an organization unit a sharing change takes the resource server from. A resource
// server keeps no values per unit, so there is nothing to delete. The change is refused while roles the unit owns
// still hold the server's permissions. Shared roles assigned in the unit, and tokens issued there, lose the server's
// permissions without a refusal.
func (d *resourceServerSharingDeclaration) DeleteOverlayValues(
	ctx context.Context, resourceID, ouID string,
) ([]string, error) {
	return nil, d.service.requireResourceServerUnusedInOU(ctx, resourceID, ouID)
}

// OnPolicyRemoved has nothing to undo: a resource server keeps no state per organization unit.
func (d *resourceServerSharingDeclaration) OnPolicyRemoved(
	context.Context, sharing.Policy,
) ([]sharing.Revocation, *tidcommon.ServiceError) {
	return nil, nil
}

// OwningOUID returns the organization unit that owns a resource server.
func (d *resourceServerSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	resourceServer, svcErr := d.service.GetResourceServer(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return resourceServer.OUID, nil
}
