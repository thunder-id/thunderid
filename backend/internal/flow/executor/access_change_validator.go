// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// accessChange is what the validator prepares for one mode: the inputs it reads and the validation it runs.
type accessChange struct {
	reason revocation.Reason
	// inputs are all required: an optional declared input would make the engine prompt for it.
	inputs         []providers.Input
	optionalArgs   []string
	targetInput    string
	assigneeInput  string
	argInputs      []string
	deploymentWide bool
	notAllowed     tidcommon.ServiceError
	validate       func(e *accessChangeValidator, ctx context.Context, inputs map[string]string) (
		*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error)
}

// accessChangesByMode maps the node's mode to the change it prepares.
var accessChangesByMode = map[string]accessChange{
	ExecutorModeRoleAssignmentRemoval: {
		reason: revocation.ReasonRoleAssignmentRemoved,
		inputs: []providers.Input{
			requiredTextInput(revocationInputRole), requiredTextInput(revocationInputAssignee),
		},
		targetInput:   revocationInputRole,
		assigneeInput: revocationInputAssignee,
		notAllowed:    ErrRoleAssignmentRemovalNotAllowed,
		validate:      (*accessChangeValidator).validateRoleAssignmentRemoval,
	},
	ExecutorModeRoleDeletion: {
		reason:      revocation.ReasonRoleDeleted,
		inputs:      []providers.Input{requiredTextInput(revocationInputRole)},
		targetInput: revocationInputRole,
		notAllowed:  ErrRoleDeletionNotAllowed,
		validate:    (*accessChangeValidator).validateRoleDeletion,
	},
	ExecutorModeRolePermissionRemoval: {
		reason: revocation.ReasonRolePermissionRemoved,
		inputs: []providers.Input{
			requiredTextInput(revocationInputRole), requiredTextInput(revocationInputPermissions),
		},
		targetInput: revocationInputRole,
		argInputs:   []string{revocationInputPermissions},
		notAllowed:  ErrRolePermissionRemovalNotAllowed,
		validate:    (*accessChangeValidator).validateRolePermissionRemoval,
	},
	ExecutorModeGroupDeletion: {
		reason:      revocation.ReasonGroupDeleted,
		inputs:      []providers.Input{requiredTextInput(revocationInputGroup)},
		targetInput: revocationInputGroup,
		notAllowed:  ErrGroupDeletionNotAllowed,
		validate:    (*accessChangeValidator).validateGroupDeletion,
	},
	ExecutorModeGroupMembershipRemoval: {
		reason: revocation.ReasonGroupMembershipRemoved,
		inputs: []providers.Input{
			requiredTextInput(revocationInputGroup), requiredTextInput(revocationInputMember),
		},
		targetInput:   revocationInputGroup,
		assigneeInput: revocationInputMember,
		notAllowed:    ErrGroupMembershipRemovalNotAllowed,
		validate:      (*accessChangeValidator).validateGroupMembershipRemoval,
	},
	ExecutorModeActionDeletion: {
		reason: revocation.ReasonActionDeleted,
		inputs: []providers.Input{
			requiredTextInput(revocationInputResourceServer), requiredTextInput(revocationInputAction),
		},
		optionalArgs:   []string{revocationInputResource},
		targetInput:    revocationInputResourceServer,
		argInputs:      []string{revocationInputAction, revocationInputResource},
		deploymentWide: true,
		notAllowed:     ErrActionDeletionNotAllowed,
		validate:       (*accessChangeValidator).validateActionDeletion,
	},
}

// accessChangeValidator validates a role, group or scope change and publishes its revocation plan.
type accessChangeValidator struct {
	providers.Executor
	roles       roleAdminProvider
	assignments roleAssignmentAdminProvider
	resources   resourceAdminProvider
}

var _ providers.Executor = (*accessChangeValidator)(nil)

// newAccessChangeValidator creates the preparatory executor for the role, group and scope flows.
func newAccessChangeValidator(factory core.FlowFactoryInterface, roles roleAdminProvider,
	assignments roleAssignmentAdminProvider, resources resourceAdminProvider) *accessChangeValidator {
	base := factory.CreateExecutor(ExecutorNameAccessChangeValidator, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			SupportedModes:     slices.Sorted(maps.Keys(accessChangesByMode)),
		})
	return &accessChangeValidator{
		Executor:    base,
		roles:       roles,
		assignments: assignments,
		resources:   resources,
	}
}

// Execute validates the change the node's mode names and publishes the plan the later nodes act on.
func (e *accessChangeValidator) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	change, ok := accessChangesByMode[ctx.ExecutorMode]
	if !ok {
		return nil, fmt.Errorf("unsupported mode for %s: %q", ExecutorNameAccessChangeValidator,
			ctx.ExecutorMode)
	}
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	inputs := make(map[string]string, len(change.inputs)+len(change.optionalArgs))
	for _, input := range change.inputs {
		value, _ := ctx.ConsumeInput(input.Identifier)
		if value == "" {
			execResp.Status = providers.ExecFailure
			execResp.Error = ErrRequiredInputEmpty.WithParams(map[string]string{"input": input.Identifier})
			return execResp, nil
		}
		inputs[input.Identifier] = value
	}
	for _, identifier := range change.optionalArgs {
		inputs[identifier], _ = ctx.ConsumeInput(identifier)
	}

	target, svcErr, err := change.validate(e, ctx.Context, inputs)
	if err != nil {
		return nil, err
	}
	if svcErr != nil {
		return refusalResponse(execResp, svcErr, change.notAllowed,
			fmt.Sprintf("failed to validate %s", change.reason))
	}

	encoded, err := encodeRevocationPlan(e.buildPlan(change, inputs, target))
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: encoded}
	return execResp, nil
}

// HasRequiredInputs prompts for the inputs of the node's mode, since each mode reads its own.
func (e *accessChangeValidator) HasRequiredInputs(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) bool {
	change, ok := accessChangesByMode[ctx.ExecutorMode]
	if !ok {
		return false
	}
	if execResp.Inputs == nil {
		execResp.Inputs = make([]providers.Input, 0, len(change.inputs))
	}
	complete := true
	for _, input := range change.inputs {
		if _, ok := ctx.UserInputs[input.Identifier]; !ok {
			execResp.Inputs = append(execResp.Inputs, input)
			complete = false
		}
	}
	return complete
}

// buildPlan pairs every affected entity with every revoked scope, or declares that nothing is revoked.
func (e *accessChangeValidator) buildPlan(change accessChange, inputs map[string]string,
	target *revocation.AccessRevocationTarget) revocationPlan {
	plan := revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   change.reason,
		Cutoff:   time.Now().UTC(),
		TargetID: inputs[change.targetInput],
	}
	if change.assigneeInput != "" {
		plan.AssigneeID = inputs[change.assigneeInput]
		if target != nil {
			plan.AssigneeType = target.AssigneeType
		}
	}
	if len(change.argInputs) > 0 {
		plan.ActionArgs = make(map[string]string, len(change.argInputs))
		for _, identifier := range change.argInputs {
			plan.ActionArgs[identifier] = inputs[identifier]
		}
	}

	if target == nil || len(target.Scopes) == 0 ||
		(!change.deploymentWide && len(target.EntityIDs) == 0) {
		plan.NothingToRevoke = true
		return plan
	}

	rows := len(target.Scopes)
	if !change.deploymentWide {
		rows *= len(target.EntityIDs)
	}
	plan.Criteria = make([]revocation.Criterion, 0, rows)
	if change.deploymentWide {
		for _, scope := range target.Scopes {
			plan.Criteria = append(plan.Criteria, revocation.Criterion{
				Type:  revocation.CriterionTypeScope,
				Value: revocation.ScopeCriterionValue(scope.Audience, scope.Scope),
			})
		}
		return plan
	}
	for _, entityID := range target.EntityIDs {
		for _, scope := range target.Scopes {
			plan.Criteria = append(plan.Criteria, revocation.Criterion{
				Type:  revocation.CriterionTypeEntityScope,
				Value: revocation.EntityScopeCriterionValue(entityID, scope.Audience, scope.Scope),
			})
		}
	}
	return plan
}

// validateRoleAssignmentRemoval validates removing the assignee from the role.
func (e *accessChangeValidator) validateRoleAssignmentRemoval(ctx context.Context,
	inputs map[string]string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error) {
	if e.assignments == nil {
		return nil, nil, errors.New("role service is not configured")
	}
	target, svcErr := e.assignments.ValidateRemoveAssignment(ctx, inputs[revocationInputRole],
		inputs[revocationInputAssignee])
	return target, svcErr, nil
}

// validateRoleDeletion validates deleting the role.
func (e *accessChangeValidator) validateRoleDeletion(ctx context.Context,
	inputs map[string]string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error) {
	if e.roles == nil {
		return nil, nil, errors.New("role service is not configured")
	}
	target, svcErr := e.roles.ValidateDeleteRole(ctx, inputs[revocationInputRole])
	return target, svcErr, nil
}

// validateRolePermissionRemoval validates replacing the role's permissions with the supplied set.
func (e *accessChangeValidator) validateRolePermissionRemoval(ctx context.Context,
	inputs map[string]string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error) {
	permissions, err := decodeRolePermissions(inputs[revocationInputPermissions])
	if err != nil {
		return nil, &ErrInvalidRolePermissions, nil
	}
	if e.roles == nil {
		return nil, nil, errors.New("role service is not configured")
	}
	target, svcErr := e.roles.ValidateUpdateRolePermissions(ctx, inputs[revocationInputRole], permissions)
	return target, svcErr, nil
}

// validateGroupDeletion validates deleting the group.
func (e *accessChangeValidator) validateGroupDeletion(ctx context.Context,
	inputs map[string]string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error) {
	if e.roles == nil {
		return nil, nil, errors.New("role service is not configured")
	}
	target, svcErr := e.roles.ValidateGroupDeletion(ctx, inputs[revocationInputGroup])
	return target, svcErr, nil
}

// validateGroupMembershipRemoval validates removing the member from the group.
func (e *accessChangeValidator) validateGroupMembershipRemoval(ctx context.Context,
	inputs map[string]string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error) {
	if e.roles == nil {
		return nil, nil, errors.New("role service is not configured")
	}
	target, svcErr := e.roles.ValidateGroupMembershipRemoval(ctx, inputs[revocationInputGroup],
		inputs[revocationInputMember])
	return target, svcErr, nil
}

// validateActionDeletion validates deleting the action, which revokes its scope deployment-wide.
func (e *accessChangeValidator) validateActionDeletion(ctx context.Context,
	inputs map[string]string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError, error) {
	if e.resources == nil {
		return nil, nil, errors.New("resource service is not configured")
	}
	target, svcErr := e.resources.ValidateDeleteAction(ctx, inputs[revocationInputResourceServer],
		optionalID(inputs[revocationInputResource]), inputs[revocationInputAction])
	return target, svcErr, nil
}
