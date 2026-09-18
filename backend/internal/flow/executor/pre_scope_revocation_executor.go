// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// defaultMaxRevocationCriteria is the fallback fan-out ceiling when a deployment configures none or a
// non-positive value, so the cap cannot be configured away. It is not the shipped default, which is
// oauth.revocation.criteria.max_criteria in default.json.
const defaultMaxRevocationCriteria = 10000

// scopeRevocationValidator validates one administrative authorization change and returns what a
// revocation against it needs. The inputs arrive keyed by identifier, so a validator reads only the ones
// its own action declares.
type scopeRevocationValidator func(ctx context.Context, inputs map[string]string) (
	*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)

// scopeActionSpec pins one preparatory node to one administrative action.
type scopeActionSpec struct {
	// name is the registered executor name, which is also what the designer palette shows.
	name string
	// reason is recorded on every row the plan writes, and is what an acting node checks the plan
	// against so a mispaired graph fails cleanly instead of revoking one thing and mutating another.
	reason revocation.Reason
	// inputs are the flow inputs the node declares and collects. All of them are required, because the
	// engine prompts for a declared optional input the caller left out, and an administration flow is
	// driven by an API caller for whom a prompt is a hang rather than a question.
	inputs []providers.Input
	// optionalArgs are arguments the node uses when the caller supplies them and does without when it
	// does not. They are deliberately not declared as node inputs, for the reason above: declaring one
	// would make the flow pause to prompt for it instead of completing in a single call.
	optionalArgs []string
	// targetInput names the input whose value the acting node operates on.
	targetInput string
	// assigneeInput names the input holding the second principal, for an action that names two
	// resources rather than one. Empty when the action names one.
	assigneeInput string
	// argInputs name the inputs the acting node needs that no criterion is derived from.
	argInputs []string
	// deploymentWide records the plan in the scope dimension rather than the per-principal one, for an
	// action that retires a scope outright instead of taking it away from someone.
	deploymentWide bool
	// notAllowed is the refusal reported when the validator gives no code of its own.
	notAllowed tidcommon.ServiceError
}

// preScopeRevocationExecutor is the preparatory node of an administration flow that takes scopes away:
// it validates the target, resolves the withdrawn scopes, and publishes the trusted revocation plan.
//
// One type backs every such node, parameterized at construction, since they differ only in inputs,
// validation and reason. They stay separately registered so a flow pins each to one action and mode.
type preScopeRevocationExecutor struct {
	providers.Executor
	spec        scopeActionSpec
	validate    scopeRevocationValidator
	maxCriteria int
}

var _ providers.Executor = (*preScopeRevocationExecutor)(nil)

// newPreScopeRevocationExecutor creates a preparatory executor pinned to one authorization change.
//
// Every such change is bounded rather than terminal: the scopes it withdraws can be regranted by
// another route, and a terminal row would then deny tokens the principal is legitimately entitled to.
// The mode is therefore fixed here and not read from the node.
func newPreScopeRevocationExecutor(factory core.FlowFactoryInterface, spec scopeActionSpec,
	maxCriteria int, validate scopeRevocationValidator) *preScopeRevocationExecutor {
	base := factory.CreateExecutor(spec.name, providers.ExecutorTypeUtility, spec.inputs, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			DefaultMode:        string(revocation.ModeBeforeAction),
			SupportedModes:     []string{string(revocation.ModeBeforeAction)},
		})
	if maxCriteria <= 0 {
		maxCriteria = defaultMaxRevocationCriteria
	}
	return &preScopeRevocationExecutor{
		Executor:    base,
		spec:        spec,
		validate:    validate,
		maxCriteria: maxCriteria,
	}
}

// Execute validates the change and publishes the plan the later nodes act on.
func (e *preScopeRevocationExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	inputs, err := e.consumeInputs(ctx)
	if err != nil {
		return nil, err
	}

	target, svcErr := e.validate(ctx.Context, inputs)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate %s: %s", e.spec.reason, svcErr.Error.DefaultValue)
		}
		// Carry the validator's own refusal rather than this executor's generic one. An unknown target, a
		// declarative one and a refused change need different things from the operator, and the console
		// resolves these codes to messages. Only a refusal with no code of its own falls back.
		execResp.Status = providers.ExecFailure
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = e.spec.notAllowed
		}
		execResp.Error = &refusal
		return execResp, nil
	}

	plan, svcErr := e.buildPlan(inputs, target)
	if svcErr != nil {
		execResp.Status = providers.ExecFailure
		execResp.Error = svcErr
		return execResp, nil
	}
	encoded, err := encodeRevocationPlan(plan)
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: encoded}
	return execResp, nil
}

// consumeInputs reads the node's declared inputs and its optional arguments. All three sources are
// read (user inputs, runtime data, forwarded data) because HasRequiredInputs accepts any of them, so
// reading only user inputs would reject a graph where an earlier node supplied the target. A present
// but empty value is an error, not a prompt: asking again would not change it.
func (e *preScopeRevocationExecutor) consumeInputs(ctx *providers.NodeContext) (
	map[string]string, error) {
	inputs := make(map[string]string, len(e.spec.inputs)+len(e.spec.optionalArgs))
	for _, input := range e.spec.inputs {
		value := e.readInput(ctx, input.Identifier)
		if value == "" {
			return nil, fmt.Errorf("%s is required", input.Identifier)
		}
		inputs[input.Identifier] = value
	}
	for _, identifier := range e.spec.optionalArgs {
		inputs[identifier] = e.readInput(ctx, identifier)
	}
	return inputs, nil
}

// readInput returns the value of one input from wherever the engine accepted it, preferring the
// caller's own input. Consuming from UserInputs records the use on the execution's audit trail, which
// is why that path goes through ConsumeInput rather than reading the map.
func (e *preScopeRevocationExecutor) readInput(ctx *providers.NodeContext, identifier string) string {
	if value, ok := ctx.ConsumeInput(identifier); ok && value != "" {
		return value
	}
	if value, ok := ctx.RuntimeData[identifier]; ok && value != "" {
		return value
	}
	if value, ok := ctx.ForwardedData[identifier]; ok {
		if text, isString := value.(string); isString {
			return text
		}
	}
	return ""
}

// buildPlan records the trusted intent for this change.
//
// A change withdrawing nothing publishes no criteria and sets NothingToRevoke, which is what keeps it
// distinguishable from a plan that lost its criteria. The cross product is deliberate: one scope for
// one principal on one resource server is the only combination a token can be matched against.
func (e *preScopeRevocationExecutor) buildPlan(inputs map[string]string,
	target *revocation.ScopeRevocationTarget) (revocationPlan, *tidcommon.ServiceError) {
	plan := revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   e.spec.reason,
		Cutoff:   time.Now().UTC(),
		TargetID: inputs[e.spec.targetInput],
	}
	if e.spec.assigneeInput != "" {
		plan.AssigneeID = inputs[e.spec.assigneeInput]
	}
	if len(e.spec.argInputs) > 0 {
		plan.ActionArgs = make(map[string]string, len(e.spec.argInputs))
		for _, identifier := range e.spec.argInputs {
			plan.ActionArgs[identifier] = inputs[identifier]
		}
	}

	if target == nil || len(target.Scopes) == 0 ||
		(!e.spec.deploymentWide && len(target.EntityIDs) == 0) {
		plan.NothingToRevoke = true
		return plan, nil
	}

	rows := len(target.Scopes)
	if !e.spec.deploymentWide {
		rows *= len(target.EntityIDs)
	}
	// Refuse before writing anything. A half-written plan leaves some principals revoked and others
	// not, with no record of where it stopped, and the window between the cutoff and the action widens
	// with every row, giving each affected principal longer to race it.
	if rows > e.maxCriteria {
		return revocationPlan{}, errRevocationFanOutTooLargeFor(rows, e.maxCriteria)
	}

	plan.Criteria = make([]revocation.Criterion, 0, rows)
	if e.spec.deploymentWide {
		for _, scope := range target.Scopes {
			plan.Criteria = append(plan.Criteria, revocation.Criterion{
				Type:  revocation.CriterionTypeScope,
				Value: revocation.ScopeCriterionValue(scope.Audience, scope.Scope),
			})
		}
		return plan, nil
	}
	for _, entityID := range target.EntityIDs {
		for _, scope := range target.Scopes {
			plan.Criteria = append(plan.Criteria, revocation.Criterion{
				Type:  revocation.CriterionTypeEntityScope,
				Value: revocation.EntityScopeCriterionValue(entityID, scope.Audience, scope.Scope),
			})
		}
	}
	return plan, nil
}

// newPreRoleAssignmentRemovalExecutor creates the preparatory executor for a role unassignment.
func newPreRoleAssignmentRemovalExecutor(factory core.FlowFactoryInterface, maxCriteria int,
	provider roleAdminProvider) *preScopeRevocationExecutor {
	spec := scopeActionSpec{
		name:   ExecutorNamePreRoleAssignmentRemoval,
		reason: revocation.ReasonRoleAssignmentRemoved,
		inputs: []providers.Input{
			textInput(revocationInputRole),
			textInput(revocationInputAssignee),
		},
		targetInput:   revocationInputRole,
		assigneeInput: revocationInputAssignee,
		notAllowed:    ErrRoleAssignmentRemovalNotAllowed,
	}
	return newPreScopeRevocationExecutor(factory, spec, maxCriteria,
		func(ctx context.Context, inputs map[string]string) (
			*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
			if provider == nil {
				return nil, internalFailure(errors.New("role service is not configured"))
			}
			return provider.ValidateRemoveRoleAssignment(ctx, inputs[revocationInputRole],
				inputs[revocationInputAssignee])
		})
}

// newPreRoleDeletionExecutor creates the preparatory executor for a role deletion.
func newPreRoleDeletionExecutor(factory core.FlowFactoryInterface, maxCriteria int,
	provider roleAdminProvider) *preScopeRevocationExecutor {
	return newPreScopeRevocationExecutor(factory, scopeActionSpec{
		name:        ExecutorNamePreRoleDeletion,
		reason:      revocation.ReasonRoleDeleted,
		inputs:      []providers.Input{textInput(revocationInputRole)},
		targetInput: revocationInputRole,
		notAllowed:  ErrRoleDeletionNotAllowed,
	}, maxCriteria, roleScopeChangeValidator(provider))
}

// newPreRolePermissionRemovalExecutor creates the preparatory executor for a role permission change.
//
// The new permission set is collected and parsed here rather than by the acting node, so the whole
// request is validated before anything is revoked. Collecting it downstream would let a caller who
// omitted or malformed it leave the flow with the scopes already denied and the role unchanged.
func newPreRolePermissionRemovalExecutor(factory core.FlowFactoryInterface, maxCriteria int,
	provider roleAdminProvider) *preScopeRevocationExecutor {
	return newPreScopeRevocationExecutor(factory, scopeActionSpec{
		name:   ExecutorNamePreRolePermissionRemoval,
		reason: revocation.ReasonRolePermissionRemoved,
		inputs: []providers.Input{
			textInput(revocationInputRole),
			textInput(revocationInputPermissions),
		},
		targetInput: revocationInputRole,
		argInputs:   []string{revocationInputPermissions},
		notAllowed:  ErrRolePermissionRemovalNotAllowed,
	}, maxCriteria, func(ctx context.Context, inputs map[string]string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
		// Parse the permission set here, before anything is revoked. The acting node decodes it again
		// to use it, but by then the deny rows are written: a malformed payload would leave every
		// holder's tokens denied and the role unchanged, with nothing to restore them.
		if _, err := decodeRolePermissions(inputs[revocationInputPermissions]); err != nil {
			return nil, ErrInvalidRolePermissions.WithParams(
				map[string]string{"reason": err.Error()})
		}
		return roleScopeChangeValidator(provider)(ctx, inputs)
	})
}

// roleScopeChangeValidator is the validation the role deletion and the role permission change share.
// Both withdraw every scope the role grants today from everyone holding it, so both ask the provider
// the same question.
func roleScopeChangeValidator(provider roleAdminProvider) scopeRevocationValidator {
	return func(ctx context.Context, inputs map[string]string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
		if provider == nil {
			return nil, internalFailure(errors.New("role service is not configured"))
		}
		return provider.ValidateRoleScopeChange(ctx, inputs[revocationInputRole])
	}
}

// newPreGroupDeletionExecutor creates the preparatory executor for a group deletion.
func newPreGroupDeletionExecutor(factory core.FlowFactoryInterface, maxCriteria int,
	provider groupAdminProvider) *preScopeRevocationExecutor {
	return newPreScopeRevocationExecutor(factory, scopeActionSpec{
		name: ExecutorNamePreGroupDeletion,
		// A group deletion and a member removal record the same reason: both cut a membership, and the
		// artifacts they deny are the same shape. Nothing downstream distinguishes them, and inventing a
		// second reason would fragment the boundary set for no gain.
		reason:      revocation.ReasonGroupMembershipRemoved,
		inputs:      []providers.Input{textInput(revocationInputGroup)},
		targetInput: revocationInputGroup,
		notAllowed:  ErrGroupDeletionNotAllowed,
	}, maxCriteria, groupMembershipChangeValidator(provider, false))
}

// newPreGroupMembershipRemovalExecutor creates the preparatory executor for removing one member from a
// group.
func newPreGroupMembershipRemovalExecutor(factory core.FlowFactoryInterface, maxCriteria int,
	provider groupAdminProvider) *preScopeRevocationExecutor {
	return newPreScopeRevocationExecutor(factory, scopeActionSpec{
		name:   ExecutorNamePreGroupMembershipRemoval,
		reason: revocation.ReasonGroupMembershipRemoved,
		inputs: []providers.Input{
			textInput(revocationInputGroup),
			textInput(revocationInputMember),
		},
		targetInput:   revocationInputGroup,
		assigneeInput: revocationInputMember,
		notAllowed:    ErrGroupMembershipRemovalNotAllowed,
	}, maxCriteria, groupMembershipChangeValidator(provider, true))
}

// groupMembershipChangeValidator is the validation the group deletion and the member removal share.
// withMember names the departing member; without it the whole group is going away and every transitive
// member loses the path through it.
func groupMembershipChangeValidator(provider groupAdminProvider,
	withMember bool) scopeRevocationValidator {
	return func(ctx context.Context, inputs map[string]string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
		if provider == nil {
			return nil, internalFailure(errors.New("group service is not configured"))
		}
		memberID := ""
		if withMember {
			memberID = inputs[revocationInputMember]
		}
		return provider.ValidateGroupMembershipChange(ctx, inputs[revocationInputGroup], memberID)
	}
}

// newPreScopeDeletionExecutor creates the preparatory executor for retiring a scope.
//
// This is the one flow whose revocation is deployment-wide. The others take a scope away from named
// principals; this one says the scope has stopped existing, so no principal should hold it and there is
// no set of holders to enumerate.
func newPreScopeDeletionExecutor(factory core.FlowFactoryInterface, maxCriteria int,
	provider resourceAdminProvider) *preScopeRevocationExecutor {
	spec := scopeActionSpec{
		name:   ExecutorNamePreScopeDeletion,
		reason: revocation.ReasonScopeDeleted,
		inputs: []providers.Input{
			textInput(revocationInputResourceServer),
			textInput(revocationInputAction),
		},
		// An action may be defined on the resource server rather than under one of its resources, and
		// the two are different actions to look up, so the resource cannot simply be assumed.
		optionalArgs:   []string{revocationInputResource},
		targetInput:    revocationInputResourceServer,
		argInputs:      []string{revocationInputAction, revocationInputResource},
		deploymentWide: true,
		notAllowed:     ErrScopeDeletionNotAllowed,
	}
	return newPreScopeRevocationExecutor(factory, spec, maxCriteria,
		func(ctx context.Context, inputs map[string]string) (
			*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
			if provider == nil {
				return nil, internalFailure(errors.New("resource service is not configured"))
			}
			return provider.ValidateDeleteAction(ctx, inputs[revocationInputResourceServer],
				inputs[revocationInputResource], inputs[revocationInputAction])
		})
}
