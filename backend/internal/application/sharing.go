// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// ApplicationSharingType is the sharing framework's identifier for applications.
const ApplicationSharingType sharing.ResourceType = resourceTypeApplication

// applicationSharingDeclaration is how an application participates in sharing: the framework asks
// it what applications are called, which fields a policy may govern, and who owns one.
//
// Sharing an application means one thing only: the named organization units may be given as the
// accessing organization unit on a token request. It does not place the application in their
// listings, and does not make it readable, editable or deletable there. That is what keeps a service
// acting on a customer's behalf invisible to that customer's own administrators.
type applicationSharingDeclaration struct {
	service ApplicationServiceInterface
}

// newApplicationSharingDeclaration returns the declaration registered with the sharing framework.
func newApplicationSharingDeclaration(service ApplicationServiceInterface) *applicationSharingDeclaration {
	return &applicationSharingDeclaration{service: service}
}

// ResourceType identifies applications to the sharing framework.
func (d *applicationSharingDeclaration) ResourceType() sharing.ResourceType {
	return ApplicationSharingType
}

// Fields declares nothing.
//
// An application's configuration, its credentials, flows and allowed grant types, is global to the
// application rather than per-organization-unit state layered on a shared definition. There is
// therefore nothing a sharee could be handed to edit, and nothing to narrow.
//
// Declaring no fields is also what makes "a sharee may not edit anything" true by construction:
// a policy can only speak about fields the type declares, so there is no rule it could carry that
// would hand any part of the application over.
func (d *applicationSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return nil
}

// OwningOUID returns the organization unit that owns an application.
//
// It reads the application rather than its OAuth client because only the former carries an owning
// organization unit; the client view has none.
func (d *applicationSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	app, svcErr := d.service.GetApplication(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return app.OUID, nil
}

// IsApplicationAccessibleFromOU reports whether an application may be used on one organization
// unit's behalf, because it owns the application or a policy reached it.
//
// Accessible from, not visible to: a shared application does not appear in the reached unit's
// listings and cannot be read or edited there. All the unit gains is the right to be named as the
// accessing organization unit on a token request.
func (as *applicationService) IsApplicationAccessibleFromOU(
	ctx context.Context, appID, ouID string,
) (bool, *tidcommon.ServiceError) {
	if appID == "" || ouID == "" || as.sharingService == nil {
		return false, nil
	}
	return as.sharingService.IsVisible(ctx, ApplicationSharingType, appID, ouID)
}

// requireAccessibleFromAccessingOU refuses a read made on behalf of an organization unit the
// application may not act for.
//
// It sits in the read rather than in the caller so that every path reaching an application inherits
// the rule. Only a request that named an organization unit is affected: an ordinary management read
// names none and is untouched.
func (as *applicationService) requireAccessibleFromAccessingOU(
	ctx context.Context, appID, owningOUID string,
) *tidcommon.ServiceError {
	accessingOUID := syscontext.GetAccessingOUID(ctx)
	if accessingOUID == "" {
		return nil
	}
	// An internal read made while the framework is deciding access must not be access-checked, or
	// resolving this application's owner would recurse back through the framework that asked.
	if security.IsRuntimeContext(ctx) {
		return nil
	}
	if owningOUID == accessingOUID {
		return nil
	}

	accessible, svcErr := as.IsApplicationAccessibleFromOU(ctx, appID, accessingOUID)
	if svcErr != nil {
		return svcErr
	}
	if !accessible {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}
