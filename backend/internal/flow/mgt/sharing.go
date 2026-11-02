// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/connection"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// FlowSharingType is the sharing framework's identifier for flows.
const FlowSharingType sharing.ResourceType = "flow"

// applicationSharingType is the sharing framework's identifier for applications, the same as
// application.ApplicationSharingType, which this package cannot import.
const applicationSharingType sharing.ResourceType = "application"

// sharingFieldKey is the sharing field the per-variable rules and values of a flow hang off. A variable's own key is
// sharingFieldKey + "." + its identifier.
const sharingFieldKey = "variables"

// sharingKey is the key of a variable in a sharing policy and in an organization unit's own values.
func sharingKey(identifier string) string {
	return sharingFieldKey + "." + identifier
}

// flowSharingDeclaration is how a flow participates in sharing.
//
// Sharing a flow lets the named organization units run it, and set the variables the flow declares: one field of
// the sharing framework per variable, "variables.<id>", so a policy can say which an organization unit may set,
// pin one to a value, or limit the values it may choose from. The field's default is the owner's own standing: the
// owner may open any variable, and a policy narrows from there. Nothing governs a variable for an organization unit
// until a policy names it, so the default is never read as terms for a unit: it has no say over what the flow's owner
// never opened to it.
type flowSharingDeclaration struct {
	store      flowStoreInterface
	sharingSvc sharing.SharingServiceInterface
}

// newFlowSharingDeclaration returns the declaration registered with the sharing framework.
func newFlowSharingDeclaration(
	store flowStoreInterface, sharingSvc sharing.SharingServiceInterface,
) *flowSharingDeclaration {
	return &flowSharingDeclaration{store: store, sharingSvc: sharingSvc}
}

// ResourceType identifies flows to the sharing framework.
func (d *flowSharingDeclaration) ResourceType() sharing.ResourceType {
	return FlowSharingType
}

// Fields declares the variables as a dynamic field. A variable's own value is a single member.
func (d *flowSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return []sharing.FieldDeclaration{
		{
			Key: sharingFieldKey, Kind: sharing.FieldScalar, DynamicKeys: true,
			Default: &sharing.OverlayRule{Editable: true},
		},
	}
}

// OwningOUID returns the organization unit that owns a flow.
func (d *flowSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	flow, err := d.store.GetFlowByID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return "", &ErrorFlowNotFound
		}
		return "", &tidcommon.InternalServerError
	}
	return flow.OUID, nil
}

// ValidateMembers refuses a connection a policy names for a connection variable when the unit making the decision
// neither owns it nor was shared it, so a unit cannot hand on a connection it cannot itself see. Only what a policy
// states is checked: a variable's default and the connections a flow already uses are not restricted by unit.
func (d *flowSharingDeclaration) ValidateMembers(
	ctx context.Context, resourceID, fieldKey, initiatingOUID string, members []string,
) *tidcommon.ServiceError {
	id, isVariable := strings.CutPrefix(fieldKey, sharingFieldKey+".")
	if !isVariable {
		return nil
	}
	flow, err := d.store.GetFlowByID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			return &ErrorFlowNotFound
		}
		return &tidcommon.InternalServerError
	}
	def, declared := variablesByID(declaredVariables(flow))[id]
	if !declared || def.Type != providers.InputTypeConnectionSelect {
		return nil
	}

	for _, member := range members {
		visible, svcErr := d.sharingSvc.IsVisible(
			security.WithRuntimeContext(ctx), connection.ConnectionSharingType, member, initiatingOUID)
		if svcErr != nil {
			return svcErr
		}
		if !visible {
			return sharingPolicyError(fmt.Sprintf(
				"the value %q of the variable %q is a connection the organization unit neither owns nor was shared",
				member, id))
		}
	}
	return nil
}
