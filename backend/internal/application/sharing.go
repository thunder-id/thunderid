// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// ApplicationSharingType is the sharing framework's identifier for applications.
const ApplicationSharingType sharing.ResourceType = resourceTypeApplication

// applicationSharingDeclaration is how an application participates in sharing.
//
// Sharing an application means one thing only: the named organization units may be given as the
// accessing organization unit on a token request. It does not place the application in their
// listings, nor make it readable or editable there.
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

// Fields declares nothing: an application's configuration is global, not per-organization-unit
// state layered on a shared definition. A policy can only govern fields the type declares, so this
// is what makes "a sharee may not edit anything" true by construction.
func (d *applicationSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return nil
}

// OwningOUID returns the organization unit that owns an application. It reads the application
// rather than its OAuth client, which carries no owning organization unit.
func (d *applicationSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	app, svcErr := d.service.GetApplication(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return app.OUID, nil
}
