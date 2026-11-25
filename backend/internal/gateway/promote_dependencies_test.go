// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promoteTarget is the gateway the suite promotes to.
const promoteTarget = "gw-2"

// dependencyFixture is a version fixture administering dev (gw-1) and prod (gw-2).
func dependencyFixture(t *testing.T) *versionFixture {
	t.Helper()
	f := newVersionFixture(t)
	prod := f.gateways.gateways[0]
	prod.ID, prod.Name = promoteTarget, testProdName
	f.gateways.gateways = append(f.gateways.gateways, prod)
	return f
}

// captureAndApply captures the documents and applies them to gw-1, with the gateway answering as given.
func (f *versionFixture) captureAndApply(t *testing.T, answer []importOutcome, docs ...string) {
	t.Helper()
	ctx := context.Background()
	f.exporter.response = documents(docs...)
	_, svcErr := f.svc.Capture(ctx, CaptureRequest{})
	require.Nil(t, svcErr)
	f.client.imported = nil
	if answer != nil {
		f.client.imported = answerOf(answer...)
	}
	_, svcErr = f.svc.Apply(ctx, "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.client.imported = nil
}

func outcomeOf(resourceType, id, status string) importOutcome {
	return importOutcome{ResourceType: resourceType, ResourceID: id, Status: status, Code: "X-1"}
}

// A revert that moves an application back to its earlier flow removes the flow the later version added:
// the application it writes no longer names that flow, so the removal is not refused as still needed.
func TestARevertRemovesTheFlowItsApplicationNoLongerNames(t *testing.T) {
	f := dependencyFixture(t)
	f.captureAndApply(t, nil, unit("ou-1"), flow("flow-a"), app("app-1", "ou-1", "flow-a"))
	f.captureAndApply(t, nil, unit("ou-1"), flow("flow-a"), flow("flow-b"), app("app-1", "ou-1", "flow-b"))

	before := len(f.client.sent)

	_, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{})

	require.Nil(t, svcErr)
	content, deletions := "", []gatewayDeletion{}
	for _, sent := range f.client.sent[before:] {
		content += sent.Content
		deletions = append(deletions, sent.Deletions...)
	}
	assert.Equal(t, []gatewayDeletion{{ResourceType: "flow", ID: "flow-b"}}, deletions)
	assert.Contains(t, content, "authFlowId: \"flow-a\"", "the application did not go back to its flow")
}

// A promotion that removes a flow the target's application still names, because the source does not run
// the application's update, is refused naming the source to deploy to first.
func TestAPromotionThatWouldLeaveAnApplicationOnARemovedFlowIsRefused(t *testing.T) {
	f := dependencyFixture(t)
	ctx := context.Background()
	f.captureAndApply(t, nil, unit("ou-1"), flow("flow-a"), app("app-1", "ou-1", "flow-a"))
	_, svcErr := f.svc.Promote(ctx, PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: promoteTarget})
	require.Nil(t, svcErr)
	f.captureAndApply(t, []importOutcome{outcomeOf("application", "app-1", outcomeFailed),
		outcomeOf("flow", "flow-b", outcomeSuccess), outcomeOf("flow", "flow-a", outcomeSuccess)},
		unit("ou-1"), flow("flow-b"), app("app-1", "ou-1", "flow-b"))
	sent := len(f.client.sent)

	_, svcErr = f.svc.Promote(ctx, PromoteRequest{SourceGatewayID: "gw-1", TargetGatewayID: promoteTarget})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorPromotionIncomplete.Code, svcErr.Code)
	assert.Equal(t, "error.gatewayservice.promotion_removal_in_use.description", svcErr.ErrorDescription.Key)
	assert.Equal(t, map[string]string{"referenced": "flow/flow-a is needed by application/app-1", "source": "dev"},
		svcErr.ErrorDescription.Params)
	assert.Len(t, f.client.sent, sent, "a refused promotion reached the gateway")
}

// A promotion of an application whose flow the source does not run is refused in a promotion's terms,
// naming the source, rather than pointing at a selection a promotion does not take.
func TestAPromotionOfAnApplicationWithoutItsFlowIsRefused(t *testing.T) {
	f := dependencyFixture(t)
	f.captureAndApply(t, []importOutcome{outcomeOf("flow", "flow-b", outcomeFailed),
		outcomeOf("application", "app-1", outcomeSuccess), outcomeOf("organization_unit", "ou-1", outcomeSuccess)},
		unit("ou-1"), flow("flow-b"), app("app-1", "ou-1", "flow-b"))

	_, svcErr := f.svc.Promote(context.Background(), PromoteRequest{SourceGatewayID: "gw-1",
		TargetGatewayID: promoteTarget})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorPromotionIncomplete.Code, svcErr.Code)
	assert.Equal(t, "error.gatewayservice.promotion_needs_unrun.description", svcErr.ErrorDescription.Key)
	assert.Equal(t, map[string]string{"missing": "application/app-1 needs flow/flow-b", "source": "dev"},
		svcErr.ErrorDescription.Params)
	assert.NotContains(t, svcErr.ErrorDescription.DefaultValue, "withDependencies")
}
