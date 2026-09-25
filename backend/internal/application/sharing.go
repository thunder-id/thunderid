// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// ApplicationSharingType is the sharing framework's identifier for applications.
const ApplicationSharingType sharing.ResourceType = "application"

// applicationSharing declares how an application participates in sharing.
//
// Sharing an application means one thing only: the named organization units may be given as the
// accessing organization unit on a token request. It does not place the application in their
// listings, and does not make it readable, editable or deletable there. That is what keeps a service
// acting on a customer's behalf invisible to that customer's own administrators.
type applicationSharing struct {
	service ApplicationServiceInterface
}

// newApplicationSharing returns the declaration registered with the sharing framework.
func newApplicationSharing(service ApplicationServiceInterface) *applicationSharing {
	return &applicationSharing{service: service}
}

// ResourceType identifies applications to the sharing framework.
func (d *applicationSharing) ResourceType() sharing.ResourceType {
	return ApplicationSharingType
}

// Fields declares nothing.
//
// An application's configuration, its credentials, flows and allowed grant types, is global to the
// application rather than per-organization-unit state layered on a shared definition. There is
// therefore nothing a sharee could be handed to edit, and nothing to narrow. What does vary per
// organization unit is which permissions the resulting token may carry, and that is derived at
// issuance from resource-server policies rather than stored against the application.
//
// Declaring no fields is also what makes "a sharee may not edit anything" true by construction:
// a policy can only speak about fields the type declares, so there is no rule it could carry that
// would hand any part of the application over.
func (d *applicationSharing) Fields() []sharing.FieldDeclaration {
	return nil
}

// OwningOUID returns the organization unit that owns an application.
//
// It reads the application rather than its OAuth client because only the former carries an owning
// organization unit; the client view has none.
func (d *applicationSharing) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	// Runtime context: the framework is resolving ownership in order to decide access, so this read
	// must not itself be access-checked, or it would recurse back through the framework.
	app, svcErr := d.service.GetApplication(security.WithRuntimeContext(ctx), resourceID)
	if svcErr != nil {
		return "", svcErr
	}
	return app.OUID, nil
}

// validateApplicationTargetScope refuses a declared policy an application may not express.
//
// The framework accepts six target scopes, but an application supports only the two blanket ones
// plus carve-outs: it is a single global registration, so "every organization unit" and "everything
// beneath mine" are the shapes that mean something, while naming individual units one hop at a time
// is the per-customer enumeration this feature exists to avoid.
//
// Rejecting here rather than letting the framework refuse later is deliberate: the framework would
// answer with a generic policy-engine error at startup, naming neither the application nor the field
// the author actually wrote.
func validateApplicationTargetScope(scope providers.SharingTargetOUScope) error {
	switch {
	case scope.AllRoots:
		return fmt.Errorf("allRoots is not supported for applications, use allOus or allChildren")
	case len(scope.RootOUIDs) > 0:
		return fmt.Errorf("rootOuIds is not supported for applications, use allOus or allChildren")
	case len(scope.ExcludedRootOUIDs) > 0:
		return fmt.Errorf("excludedRootOuIds is not supported for applications, use excludedOuIds")
	case len(scope.OUIDs) > 0:
		return fmt.Errorf("ouIds is not supported for applications, use allOus or allChildren")
	}

	if scope.AllOUs == scope.AllChildren {
		return fmt.Errorf("exactly one of allOus or allChildren is required")
	}
	return nil
}

// validateApplicationPolicy refuses a declared policy an application cannot carry.
func validateApplicationPolicy(p providers.SharingPolicy) error {
	if len(p.OverlayRules) > 0 {
		return fmt.Errorf(
			"overlayRules are not supported for applications: an application shares no editable field")
	}
	return validateApplicationTargetScope(p.TargetOuScope)
}

// IsApplicationVisibleToOU reports whether an application may be used on one organization unit's
// behalf, because it owns the application or a policy reached it.
func (as *applicationService) IsApplicationVisibleToOU(
	ctx context.Context, appID, ouID string,
) (bool, *tidcommon.ServiceError) {
	if appID == "" || ouID == "" || as.sharingService == nil {
		return false, nil
	}
	return as.sharingService.IsVisible(ctx, ApplicationSharingType, appID, ouID)
}

// declaredAppPolicies is one application's declared policies, captured while its file was parsed.
//
// The name is carried alongside the id because a startup failure has to say which file is wrong, and
// an id alone sends the operator looking through every document in the directory.
type declaredAppPolicies struct {
	appID    string
	appName  string
	ownerOU  string
	policies []providers.SharingPolicy
}

// seedDeclaredApplicationPolicies replays the policies declared across the loaded batch.
//
// They are declared, not stored: the file is the only source of truth and there is no API to edit
// them through. That is what makes replay need no deduplication of its own. The framework holds
// declared policies in memory, so every start begins with an empty store and re-seeds the files into
// it; nothing accumulates across restarts because nothing survives one.
//
// Within a single start the framework still refuses a second policy for the same initiating
// organization unit, and that refusal is left fatal rather than swallowed: reaching it means two
// documents declare the same thing, which is an authoring mistake worth stopping for.
func seedDeclaredApplicationPolicies(
	declared []declaredAppPolicies, sharingService sharing.ServiceInterface,
) error {
	if len(declared) == 0 {
		return nil
	}
	if sharingService == nil {
		return fmt.Errorf(
			"application %q declares sharing policies but the sharing framework is not enabled",
			declared[0].appName)
	}

	// Seeding carries no caller to scope against.
	ctx := security.WithRuntimeContext(context.Background())

	for _, d := range declared {
		for i, policy := range d.policies {
			initiator := policy.InitiatingOuID
			if initiator == "" {
				initiator = d.ownerOU
			}
			_, svcErr := sharingService.CreateDeclarativePolicy(
				ctx, ApplicationSharingType, d.appID, d.ownerOU, sharing.RequestFromDeclaration(policy))
			if svcErr == nil {
				continue
			}
			return fmt.Errorf(
				"failed to declare sharing policy %d of application %q (initiating organization unit %q): %s: %s",
				i+1, d.appName, initiator, svcErr.Code, svcErr.ErrorDescription.DefaultValue)
		}
	}
	return nil
}
