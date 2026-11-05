// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// roleAction is the role change made for one mode and the refusal reported when the service names none.
type roleAction struct {
	reason revocation.Reason
	failed tidcommon.ServiceError
	act    func(ctx context.Context, e *roleExecutor, plan revocationPlan) (*tidcommon.ServiceError, error)
}

// roleActionsByMode maps the node's mode to the role change it makes.
var roleActionsByMode = map[string]roleAction{
	ExecutorModeRolePermissionRemoval: {
		reason: revocation.ReasonRolePermissionRemoved,
		failed: ErrRolePermissionRemovalFailed,
		act: func(ctx context.Context, e *roleExecutor, plan revocationPlan) (*tidcommon.ServiceError, error) {
			permissions, err := decodeRolePermissions(plan.ActionArgs[revocationInputPermissions])
			if err != nil {
				return nil, err
			}
			current, svcErr := e.roles.GetRoleWithPermissions(ctx, plan.TargetID)
			if svcErr != nil {
				return svcErr, nil
			}
			// The update replaces the whole role, so the attributes not being changed are carried over.
			_, svcErr = e.roles.UpdateRoleWithPermissions(ctx, plan.TargetID, role.RoleUpdateDetail{
				Name:        current.Name,
				Description: current.Description,
				OUID:        current.OUID,
				Permissions: permissions,
			})
			return svcErr, nil
		},
	},
}

// roleModeFlowTypes maps each mode to the flow type it serves: the revocation plan modes administration flows,
// and role selection and assignment invitation flows.
var roleModeFlowTypes = map[string][]providers.FlowType{
	ExecutorModeRoleAssignment:        {providers.FlowTypeInvitation},
	ExecutorModeRolePermissionRemoval: {providers.FlowTypeAdministration},
	ExecutorModeRoleSelection:         {providers.FlowTypeInvitation},
}

// recordedCaller is the admin an invitation flow records, to assign its roles as at acceptance.
type recordedCaller struct {
	Subject     string   `json:"subject"`
	OUID        string   `json:"ouId"`
	Permissions []string `json:"permissions"`
}

// roleOption is a role a role selection prompt offers.
type roleOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// roleExecutor changes roles. In an administration flow it performs the role change named in the trusted
// revocation plan; in an invitation flow it records the roles an admin picks and assigns them as that admin.
type roleExecutor struct {
	providers.Executor
	roles       roleAdminProvider
	assignments roleAssignmentAdminProvider
	permissions entityPermissionResolver
	logger      *log.Logger
}

var _ providers.Executor = (*roleExecutor)(nil)

// newRoleExecutor creates the executor that changes roles in administration and invitation flows.
func newRoleExecutor(factory core.FlowFactoryInterface, roles roleAdminProvider,
	assignments roleAssignmentAdminProvider, permissions entityPermissionResolver) *roleExecutor {
	base := factory.CreateExecutor(ExecutorNameRole, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedModes:     slices.Sorted(maps.Keys(roleModeFlowTypes)),
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration, providers.FlowTypeInvitation},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyRoles},
			},
		})
	return &roleExecutor{
		Executor:    base,
		roles:       roles,
		assignments: assignments,
		permissions: permissions,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, ExecutorNameRole),
			log.String(log.LoggerKeyExecutorName, ExecutorNameRole)),
	}
}

// Execute performs the role change of the node's mode.
func (e *roleExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	mode := ctx.ExecutorMode
	flowTypes, ok := roleModeFlowTypes[mode]
	if !ok {
		return nil, fmt.Errorf("unsupported mode for %s: %q", ExecutorNameRole, ctx.ExecutorMode)
	}
	if !slices.Contains(flowTypes, ctx.FlowType) {
		return nil, fmt.Errorf("mode %q of %s is not supported in %s flows", mode, ExecutorNameRole, ctx.FlowType)
	}
	switch mode {
	case ExecutorModeRoleSelection, ExecutorModeRoleAssignment:
		if e.assignments == nil {
			return nil, errors.New("role assignment service is not configured")
		}
		if mode == ExecutorModeRoleSelection {
			return e.selectRoles(ctx)
		}
		return e.assign(ctx)
	default:
		return e.applyRevocationPlan(ctx, roleActionsByMode[mode])
	}
}

// applyRevocationPlan performs the change identified by the trusted revocation plan.
func (e *roleExecutor) applyRevocationPlan(ctx *providers.NodeContext,
	action roleAction) (*providers.ExecutorResponse, error) {
	if e.roles == nil {
		return nil, errors.New("role service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, action.reason)
	if err != nil {
		return nil, err
	}

	svcErr, err := action.act(ctx.Context, e, plan)
	if err != nil {
		return nil, err
	}
	if svcErr != nil {
		return refusalResponse(&providers.ExecutorResponse{}, svcErr, action.failed,
			fmt.Sprintf("failed to perform %s", action.reason))
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

// selectRoles records the roles the node names, or else prompts for roles the caller may assign in the flow's unit,
// and records the picked ones and the caller.
func (e *roleExecutor) selectRoles(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	if !isInviter(ctx) {
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &ErrNotTheInviter}, nil
	}
	ouID := ctx.RuntimeData[common.RuntimeKeyOUID]
	if ouID == "" {
		return nil, errors.New("the flow's organization unit is not set")
	}
	configured, err := runtimeProperty(ctx, propertyKeyRoles)
	if err != nil {
		return nil, err
	}
	assignable, svcErr := e.assignments.ListAssignableRoles(ctx.Context, ouID)
	if svcErr != nil {
		return refusalResponse(&providers.ExecutorResponse{}, svcErr, ErrRoleNotAssignable,
			"failed to list assignable roles")
	}
	isAssignable := func(id string) bool {
		return slices.ContainsFunc(assignable, func(r role.Role) bool { return r.ID == id })
	}

	// Roles the node names are not the admin's to change, so one the admin may not assign fails the step.
	if configured != "" {
		selected := uniqueFields(configured)
		if !allOf(selected, isAssignable) {
			return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &ErrRoleNotAssignable}, nil
		}
		return recordSelection(ctx, selected)
	}

	// With nothing the caller may assign, the prompt could never be answered.
	if len(assignable) == 0 {
		return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
	}
	selected := uniqueFields(ctx.UserInputs[userInputRoles])
	if len(selected) == 0 {
		return rolePrompt(assignable, nil)
	}
	if !allOf(selected, isAssignable) {
		return rolePrompt(assignable, &ErrRoleNotAssignable)
	}
	return recordSelection(ctx, selected)
}

// uniqueFields splits a space-separated list, dropping repeats and keeping the first occurrence's order.
func uniqueFields(value string) []string {
	var unique []string
	for _, field := range strings.Fields(value) {
		if !slices.Contains(unique, field) {
			unique = append(unique, field)
		}
	}
	return unique
}

// allOf reports whether every id satisfies ok.
func allOf(ids []string, ok func(string) bool) bool {
	for _, id := range ids {
		if !ok(id) {
			return false
		}
	}
	return true
}

// recordSelection records the selected roles and the caller who selected them, to assign them as at acceptance.
func recordSelection(ctx *providers.NodeContext, selected []string) (*providers.ExecutorResponse, error) {
	caller, err := json.Marshal(recordedCaller{
		Subject:     security.GetSubject(ctx.Context),
		OUID:        security.GetOUID(ctx.Context),
		Permissions: security.GetPermissions(ctx.Context),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record the inviting caller: %w", err)
	}
	return &providers.ExecutorResponse{
		Status: providers.ExecComplete,
		RuntimeData: map[string]string{
			common.RuntimeKeySelectedRoles:   strings.Join(selected, " "),
			common.RuntimeKeyRolesSelectedBy: string(caller),
		},
	}, nil
}

// heldPermissions returns the recorded admin's permissions that they still hold on some resource server. The record
// is a copy of the admin's token: a permission revoked since then is left out, and so are scopes no resource server
// holds, such as openid.
func (e *roleExecutor) heldPermissions(ctx context.Context, caller recordedCaller) ([]string, error) {
	if e.permissions == nil {
		return nil, errors.New("permission resolver is not configured")
	}
	current, err := e.permissions.ResolveForEntity(ctx, caller.Subject)
	if err != nil {
		return nil, err
	}
	held := make([]string, 0, len(caller.Permissions))
	for _, permission := range caller.Permissions {
		for _, onServer := range current {
			if security.HasSufficientPermission(onServer, permission) {
				held = append(held, permission)
				break
			}
		}
	}
	return held, nil
}

// rolePrompt asks for one or more of the assignable roles, with the refusal that sent the prompt back if any.
func rolePrompt(assignable []role.Role, refusal *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
	ids := make([]string, 0, len(assignable))
	options := make([]roleOption, 0, len(assignable))
	for _, r := range assignable {
		ids = append(ids, r.ID)
		options = append(options, roleOption{ID: r.ID, Name: r.Name})
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the role options: %w", err)
	}
	input := providers.Input{
		Identifier: userInputRoles, Type: providers.InputTypeRoleSelect, Required: true, Options: ids,
	}
	return &providers.ExecutorResponse{
		Status:         providers.ExecUserInputRequired,
		Inputs:         []providers.Input{input},
		AdditionalData: map[string]string{common.DataRoleOptions: string(optionsJSON)},
		ForwardedData:  map[string]interface{}{common.ForwardedDataKeyInputs: []providers.Input{input}},
		Error:          refusal,
	}, nil
}

// assign assigns the recorded roles to the user in the flow's unit, as the recorded admin. Every role is
// assigned or none is, and the share the flow made stays either way.
func (e *roleExecutor) assign(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	roleIDs := strings.Fields(ctx.RuntimeData[common.RuntimeKeySelectedRoles])
	if len(roleIDs) == 0 {
		return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
	}
	failed := &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &ErrRoleAssignmentFailed}
	var caller recordedCaller
	if err := json.Unmarshal([]byte(ctx.RuntimeData[common.RuntimeKeyRolesSelectedBy]), &caller); err != nil ||
		caller.Subject == "" {
		return failed, nil
	}
	ouID, userID := ctx.RuntimeData[common.RuntimeKeyOUID], ctx.RuntimeData[userAttributeUserID]
	if ouID == "" || userID == "" {
		return nil, errors.New("the flow's organization unit or user is not set")
	}

	held, err := e.heldPermissions(ctx.Context, caller)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the recorded admin's permissions: %w", err)
	}
	if len(held) == 0 {
		e.logger.Debug(ctx.Context, "The recorded admin no longer holds any permission the roles were selected with")
		return failed, nil
	}

	// The role service checks the grant with what the admin still holds.
	adminCtx := security.WithRecordedCaller(ctx.Context, caller.Subject, caller.OUID, held)
	assignee := []role.RoleAssignment{{ID: userID, Type: role.AssigneeTypeUser, OUID: ouID}}
	if svcErr := e.assignments.AddAssigneesToRoles(adminCtx, assignee, roleIDs); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to assign the selected roles: %s", svcErr.Error.DefaultValue)
		}
		return failed, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}
