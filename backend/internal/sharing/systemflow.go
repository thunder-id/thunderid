// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// The system flows a sharing change runs around itself to revoke what was issued in the organization units it takes
// a resource away from, and the inputs they read.
const (
	// systemFlowRevocationPost follows every pre flow, moving its cutoff past the write.
	systemFlowRevocationPost = "system-revocation-post"
	// systemFlowInputOU names the organization unit the resource was unshared from.
	systemFlowInputOU = "targetOuId"
	// systemFlowInputOUs lists, as a JSON array, the organization units the resource was unshared from, for a flow
	// that covers them all at once.
	systemFlowInputOUs = "targetOuIds"
	// systemFlowInputRemovedHolders lists, as a JSON array, the entities whose access in the unit the change took
	// away, for the post flow to revoke any its pre flow did not see.
	systemFlowInputRemovedHolders = "removedHolderIds"
)

// systemFlowProvider runs a system flow to completion. The flow engine implements it.
type systemFlowProvider interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// cachesReads reports whether what ctx reads may be cached: not in a run that is rolled back, which reads a
// state no other request should see.
func cachesReads(ctx context.Context) bool {
	c, _ := ctx.Value(lossCollectorKey{}).(*lossCollector)
	return c == nil || !c.dryRun
}

// key identifies the revocation by its flow and inputs; a map's keys are encoded in order.
func (r Revocation) key() string {
	encoded, _ := json.Marshal(r)
	return string(encoded)
}

// requireRevocations records, during a sharing change that revokes, the revocations a resource type's hook answered
// with for state it removed in a unit that keeps the resource. They run before the change is made, and a change
// requiring one its first run did not is refused. Outside such a change nothing is recorded.
func requireRevocations(ctx context.Context, revocations []Revocation) error {
	c, _ := ctx.Value(lossCollectorKey{}).(*lossCollector)
	if c == nil {
		return nil
	}
	for _, r := range revocations {
		key := r.key()
		if c.plannedRequired != nil {
			if _, ok := c.plannedRequired[key]; !ok {
				c.unplanned = true
				return errUnplannedLoss
			}
		}
		if c.required == nil {
			c.required = map[string]Revocation{}
		}
		c.required[key] = r
	}
	return nil
}

// recordRemovedHolders records, with the collector the context carries, the entities whose access to the changed
// policy's resource in ouID the change's cleanup took away.
func recordRemovedHolders(ctx context.Context, changed Policy, ouID string, holders []string) {
	c, _ := ctx.Value(lossCollectorKey{}).(*lossCollector)
	if c == nil || len(holders) == 0 || c.rt != changed.ResourceType || c.resourceID != changed.ResourceID {
		return
	}
	if c.removedHolders == nil {
		c.removedHolders = map[string][]string{}
	}
	c.removedHolders[ouID] = append(c.removedHolders[ouID], holders...)
}

// recordLoss records that the changed policy's resource lost ouID with the collector the context carries, and
// refuses a loss the collector did not plan for.
func recordLoss(ctx context.Context, changed Policy, ouID string) error {
	c, _ := ctx.Value(lossCollectorKey{}).(*lossCollector)
	if c == nil || c.rt != changed.ResourceType || c.resourceID != changed.ResourceID {
		return nil
	}
	c.owningOUID = changed.OwningOUID
	if c.planned != nil {
		if _, ok := c.planned[ouID]; !ok {
			c.unplanned = true
			return errUnplannedLoss
		}
	}
	if !slices.Contains(c.lost, ouID) {
		c.lost = append(c.lost, ouID)
	}
	return nil
}

// unshareRevocation is one pre flow run: the inputs its flows run with and what it shared.
type unshareRevocation struct {
	inputs map[string]string
	shared map[string]string
}

// revokeAround runs change so that what was issued in the organization units it takes the resource away from is
// revoked, along with what its hooks require. change runs once in a transaction, to learn those units and
// revocations, and that run is kept when it takes the resource from no unit and requires nothing. Otherwise it is
// rolled back; the resource type's pre flow runs in each unit, and each required pre flow; then change runs again,
// refused if it takes the resource from a unit, or requires a revocation, the first run did not; then the post flows
// run. change reports whether it was made, and keeps its own result and refusal. A type that revokes nothing, or a
// server without system flows, runs change alone.
func (s *sharingService) revokeAround(ctx context.Context, rt ResourceType, resourceID string,
	change func(context.Context) bool) *tidcommon.ServiceError {
	revoker, ok := s.unshareRevoker(rt)
	if !ok || s.systemFlows == nil {
		change(ctx)
		return nil
	}
	flow := revoker.UnshareRevocationFlow()

	probe := &lossCollector{rt: rt, resourceID: resourceID, dryRun: true}
	made, kept := false, false
	// What the change records for a replay reaches the caller only from the run that is kept.
	report := &ReplayReport{}
	probeCtx := context.WithValue(context.WithValue(ctx, lossCollectorKey{}, probe), replayReportKey{}, report)
	err := s.transactioner.Transact(probeCtx, func(txCtx context.Context) error {
		made = change(txCtx)
		if made && len(probe.lost) == 0 && len(probe.required) == 0 {
			kept = true
			return nil
		}
		return errDryRun
	})
	s.clearCaches(ctx)
	if kept && err == nil {
		RecordRemoved(ctx, report.Removed())
		return nil
	}
	if !errors.Is(err, errDryRun) {
		s.logger.Error(ctx, "Failed to work out what a sharing change revokes", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if !made {
		return nil
	}

	// Cleanup visits every unit the old reach could have covered; only one that saw the resource holds anything.
	held, svcErr := s.unitsThatSawResource(ctx, rt, resourceID, probe.owningOUID, probe.lost)
	if svcErr != nil {
		return svcErr
	}
	pending := make([]Revocation, 0, len(held)+len(probe.required))
	switch {
	case flow.EveryUnit && len(held) > 0:
		// A list of strings always encodes.
		encoded, _ := json.Marshal(held)
		pending = append(pending, Revocation{PreFlow: flow.Handle,
			Inputs: map[string]string{flow.ResourceInput: resourceID, systemFlowInputOUs: string(encoded)}})
	case !flow.EveryUnit:
		for _, ouID := range held {
			pending = append(pending, Revocation{PreFlow: flow.Handle,
				Inputs: map[string]string{flow.ResourceInput: resourceID, systemFlowInputOU: ouID}})
		}
	}
	for _, key := range slices.Sorted(maps.Keys(probe.required)) {
		pending = append(pending, probe.required[key])
	}
	revocations := make([]unshareRevocation, 0, len(pending))
	for _, r := range pending {
		shared, svcErr := s.runSystemFlow(ctx, r.PreFlow, r.Inputs, nil)
		if svcErr != nil {
			return svcErr
		}
		revocations = append(revocations, unshareRevocation{inputs: r.Inputs, shared: shared})
	}

	committed := &lossCollector{rt: rt, resourceID: resourceID, planned: make(map[string]struct{}, len(probe.lost)),
		plannedRequired: make(map[string]struct{}, len(probe.required))}
	for _, ouID := range probe.lost {
		committed.planned[ouID] = struct{}{}
	}
	for key := range probe.required {
		committed.plannedRequired[key] = struct{}{}
	}
	made = change(context.WithValue(ctx, lossCollectorKey{}, committed))
	if committed.unplanned {
		return &ErrorSharingChangedDuringRevocation
	}
	if !made {
		return nil
	}

	// Every post flow runs, so one unit's failure does not leave another's revocation unfinished. A unit's post flow
	// also revokes the holders the change took away there, which its pre flow may not have seen.
	var postErr *tidcommon.ServiceError
	for _, revoked := range revocations {
		inputs := revoked.inputs
		if holders := committed.removedHolders[inputs[systemFlowInputOU]]; len(holders) > 0 {
			inputs = maps.Clone(inputs)
			// A list of strings always encodes.
			encoded, _ := json.Marshal(holders)
			inputs[systemFlowInputRemovedHolders] = string(encoded)
		}
		if _, svcErr := s.runSystemFlow(ctx, systemFlowRevocationPost, inputs, revoked.shared); svcErr != nil {
			s.logger.Error(ctx, "Failed to run the revocation post flow", log.String("resourceType", string(rt)),
				log.String("code", svcErr.Code))
			if postErr == nil {
				postErr = svcErr
			}
		}
	}
	return postErr
}

// unitsThatSawResource returns, sorted, the units of ouIDs that see the resource as it stands. A unit deleted while
// a policy still names it is kept, since what was issued there before is still to be revoked.
func (s *sharingService) unitsThatSawResource(ctx context.Context, rt ResourceType, resourceID, owningOUID string,
	ouIDs []string) ([]string, *tidcommon.ServiceError) {
	held := make([]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		visible, _, svcErr := s.resolveVisibility(ctx, rt, resourceID, owningOUID, ouID)
		switch {
		case svcErr != nil && svcErr.Code != oupkg.ErrorOrganizationUnitNotFound.Code:
			s.logger.Error(ctx, "Failed to resolve what a sharing change revokes", log.String("ouID", ouID),
				log.String("code", svcErr.Code))
			return nil, &tidcommon.InternalServerError
		case svcErr != nil || visible:
			held = append(held, ouID)
		}
	}
	slices.Sort(held)
	return held, nil
}

// unshareRevoker returns the resource type's unshare revocation, when it has one.
func (s *sharingService) unshareRevoker(rt ResourceType) (UnshareRevoker, bool) {
	decl, ok := s.registry.get(rt)
	if !ok {
		return nil, false
	}
	revoker, ok := decl.(UnshareRevoker)
	return revoker, ok
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and returns what
// this run shared.
func (s *sharingService) runSystemFlow(ctx context.Context, handle string,
	inputs, runtimeData map[string]string) (map[string]string, *tidcommon.ServiceError) {
	result, svcErr := s.systemFlows.ExecuteSystemFlow(ctx, &flowcommon.SystemFlowContext{
		Handle: handle, Inputs: inputs, RuntimeData: runtimeData})
	if svcErr != nil {
		return nil, svcErr
	}
	return result.RuntimeData, nil
}
