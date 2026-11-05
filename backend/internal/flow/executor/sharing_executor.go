// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// sharingModeFlowTypes maps each mode to the flow types it runs in: validate only in an invitation flow, and share
// there or in a sign-in or registration flow.
var sharingModeFlowTypes = map[string][]providers.FlowType{
	ExecutorModeShare: {providers.FlowTypeInvitation, providers.FlowTypeAuthentication,
		providers.FlowTypeRegistration},
	ExecutorModeValidate: {providers.FlowTypeInvitation},
}

// sharingExecutor authorizes a share into a child organization unit in the admin's half of a user
// invitation flow, and applies it on the owner's authority in the invitee's half. In a sign-in or registration
// flow it applies the share its properties name, on the owner's authority too.
type sharingExecutor struct {
	providers.Executor
	sharing sharingAdminProvider
}

var _ providers.Executor = (*sharingExecutor)(nil)

// newSharingExecutor creates the executor that shares a resource into the flow's organization unit.
func newSharingExecutor(factory core.FlowFactoryInterface, sharingSvc sharingAdminProvider) *sharingExecutor {
	base := factory.CreateExecutor(ExecutorNameSharing, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedModes: slices.Sorted(maps.Keys(sharingModeFlowTypes)),
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeInvitation, providers.FlowTypeAuthentication,
				providers.FlowTypeRegistration},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyResourceType, IsRequired: true},
				{Property: propertyKeyResourceID},
				{Property: propertyKeyOwningOUID},
				{Property: propertyKeyTargetOUID, IsRequired: true},
			},
		})
	return &sharingExecutor{Executor: base, sharing: sharingSvc}
}

// Execute authorizes the share in validate mode and applies it in share mode.
func (e *sharingExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if e.sharing == nil {
		return nil, errors.New("sharing service is not configured")
	}
	if flowTypes, ok := sharingModeFlowTypes[ctx.ExecutorMode]; ok && !slices.Contains(flowTypes, ctx.FlowType) {
		return nil, fmt.Errorf("mode %q of %s does not run in a %s flow", ctx.ExecutorMode, ExecutorNameSharing,
			ctx.FlowType)
	}
	switch ctx.ExecutorMode {
	case ExecutorModeValidate:
		return e.validate(ctx)
	case ExecutorModeShare:
		return e.share(ctx)
	default:
		return nil, fmt.Errorf("unsupported mode for %s: %q", ExecutorNameSharing, ctx.ExecutorMode)
	}
}

// validate authorizes the share as the caller who started the flow and records what it authorized.
func (e *sharingExecutor) validate(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if !isInviter(ctx) {
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &ErrNotTheInviter}, nil
	}
	resourceType, targetOUID, err := shareTarget(ctx)
	if err != nil {
		return nil, err
	}
	resourceID, err := runtimeProperty(ctx, propertyKeyResourceID)
	if err != nil {
		return nil, err
	}
	owningOUID, err := runtimeProperty(ctx, propertyKeyOwningOUID)
	if err != nil {
		return nil, err
	}

	if svcErr := e.sharing.ValidateShareIntoChildOU(
		ctx.Context, sharing.ResourceType(resourceType), resourceID, owningOUID, targetOUID); svcErr != nil {
		return refusalResponse(&providers.ExecutorResponse{}, svcErr, ErrShareFailed, "failed to validate share")
	}
	return &providers.ExecutorResponse{
		Status:      providers.ExecComplete,
		RuntimeData: map[string]string{common.RuntimeKeyAuthorizedShare: resourceType + ":" + targetOUID},
	}, nil
}

// share applies, in an invitation flow, the share validate recorded in it and no other; elsewhere, the share its
// properties name.
func (e *sharingExecutor) share(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	resourceType, targetOUID, err := shareTarget(ctx)
	if err != nil {
		return nil, err
	}
	invitation := ctx.FlowType == providers.FlowTypeInvitation
	if invitation && ctx.RuntimeData[common.RuntimeKeyAuthorizedShare] != resourceType+":"+targetOUID {
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &ErrShareNotAuthorized}, nil
	}
	resourceID, err := runtimeProperty(ctx, propertyKeyResourceID)
	if err != nil {
		return nil, err
	}
	if resourceID == "" {
		return nil, fmt.Errorf("property %q is required to share", propertyKeyResourceID)
	}

	if svcErr := e.sharing.ShareIntoChildOU(ctx.Context, sharing.ResourceType(resourceType), resourceID,
		targetOUID); svcErr != nil {
		return refusalResponse(&providers.ExecutorResponse{}, svcErr, ErrShareFailed, "failed to share")
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

// shareTarget resolves the required resource type and target organization unit.
func shareTarget(ctx *providers.NodeContext) (string, string, error) {
	resourceType, err := runtimeProperty(ctx, propertyKeyResourceType)
	if err != nil {
		return "", "", err
	}
	targetOUID, err := runtimeProperty(ctx, propertyKeyTargetOUID)
	if err != nil {
		return "", "", err
	}
	if resourceType == "" || targetOUID == "" {
		return "", "", fmt.Errorf("properties %q and %q are required", propertyKeyResourceType, propertyKeyTargetOUID)
	}
	return resourceType, targetOUID, nil
}
