// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// entityRevocation is what the pre-revocation executor revokes for one node mode.
type entityRevocation struct {
	reason revocation.Reason
	// byClient revokes what the entity's OAuth client issued. Otherwise the plan revokes what was issued to
	// the entity as a subject.
	byClient bool
}

// entityRevocationModes lists the executor's modes in the order it reports them.
var entityRevocationModes = []string{ExecutorModeEntityRemoval, ExecutorModeClientRemoval,
	ExecutorModeClientSecretRotation, ExecutorModeCredentialUpdate}

// entityRevocationsByMode maps each mode to the change it plans for, and the reason it is recorded under. The
// reasons name the change, not the kind of entity, since nothing reads the kind from them.
var entityRevocationsByMode = map[string]entityRevocation{
	ExecutorModeEntityRemoval:        {reason: revocation.ReasonEntityRemoved},
	ExecutorModeCredentialUpdate:     {reason: revocation.ReasonCredentialUpdated},
	ExecutorModeClientRemoval:        {reason: revocation.ReasonClientRemoved, byClient: true},
	ExecutorModeClientSecretRotation: {reason: revocation.ReasonClientSecretRotated, byClient: true},
}

// preRevocationExecutor is the preparatory node of the system flows run when an entity, or the OAuth client
// it acts through, is removed or has a credential replaced. Its mode names the change and so what the plan
// revokes. The service making the change has already decided that it may be made, so the node only plans.
type preRevocationExecutor struct {
	providers.Executor
	actors           providers.ActorProvider
	artifactLifetime artifactLifetimeResolver
	entities         entityprovider.EntityProviderInterface
}

var _ providers.Executor = (*preRevocationExecutor)(nil)

// newPreRevocationExecutor creates the executor that plans the entity and client revocation flows.
func newPreRevocationExecutor(factory core.FlowFactoryInterface,
	artifactLifetime artifactLifetimeResolver, entities entityprovider.EntityProviderInterface) *preRevocationExecutor {
	base := factory.CreateExecutor(ExecutorNamePreRevocation, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputEntity, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeSystem},
			SupportedModes:     entityRevocationModes,
		})
	return &preRevocationExecutor{Executor: base, artifactLifetime: artifactLifetime, entities: entities}
}

// setActorProvider injects the actor provider. See SetActorProvider.
func (e *preRevocationExecutor) setActorProvider(provider providers.ActorProvider) {
	e.actors = provider
}

// Execute publishes the plan for the change the node's mode names, bounded at now. The post flow moves the
// bound past the change once it is stored.
func (e *preRevocationExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	change, ok := entityRevocationsByMode[ctx.ExecutorMode]
	if !ok {
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &ErrInvalidRevocationMode}, nil
	}
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	entityID, ok := ctx.ConsumeInput(revocationInputEntity)
	if !ok || entityID == "" {
		return nil, fmt.Errorf("target entity is required")
	}

	plan := revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Cutoff:   time.Now().UTC(),
		Reason:   change.reason,
		TargetID: entityID,
	}
	if change.byClient {
		if err := e.planForClient(ctx, entityID, &plan); err != nil {
			return nil, err
		}
	} else {
		plan.Criteria = []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: entityID}}
	}

	encoded, err := encodeRevocationPlan(plan)
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: encoded}
	return execResp, nil
}

// planForClient points the plan at the entity's OAuth client, kept for as long as an artifact issued to it can
// live. A client already removed, as on a retry after the step that followed its deletion failed, is still
// planned from the client ID the entity records, kept for the default time since its lifetime went with it. An
// entity with no OAuth client has nothing issued through one, which the plan records explicitly.
func (e *preRevocationExecutor) planForClient(ctx *providers.NodeContext, entityID string,
	plan *revocationPlan) error {
	if e.actors == nil {
		return fmt.Errorf("actor provider is not configured")
	}
	if e.artifactLifetime == nil {
		return fmt.Errorf("artifact lifetime resolver is not configured")
	}
	client, svcErr := e.actors.GetOAuthClientByID(ctx.Context, entityID)
	if svcErr != nil {
		return fmt.Errorf("failed to resolve the entity's OAuth client: %s", svcErr.Error.DefaultValue)
	}
	if client != nil && client.ClientID != "" {
		plan.Criteria = []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: client.ClientID}}
		plan.TTLSeconds = int64(e.artifactLifetime(client) / time.Second)
		return nil
	}
	clientID, err := e.recordedClientID(ctx, entityID)
	if err != nil {
		return err
	}
	if clientID == "" {
		plan.NothingToRevoke = true
		return nil
	}
	plan.Criteria = []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: clientID}}
	return nil
}

// recordedClientID returns the OAuth client ID the entity's system attributes record, or an empty string when
// the entity is gone or records none.
func (e *preRevocationExecutor) recordedClientID(ctx *providers.NodeContext, entityID string) (string, error) {
	if e.entities == nil {
		return "", nil
	}
	entity, epErr := e.entities.GetEntityProfile(ctx.Context, entityID)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return "", nil
		}
		return "", fmt.Errorf("failed to read the entity's client ID: %s", epErr.Error())
	}
	var attrs struct {
		ClientID string `json:"clientId"`
	}
	if entity == nil || len(entity.SystemAttributes) == 0 ||
		json.Unmarshal(entity.SystemAttributes, &attrs) != nil {
		return "", nil
	}
	return attrs.ClientID, nil
}
