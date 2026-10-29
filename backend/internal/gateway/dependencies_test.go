// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/export"
)

// documents is an export of the given documents.
func documents(docs ...string) *export.ExportResponse {
	response := &export.ExportResponse{}
	for _, doc := range docs {
		response.Files = append(response.Files, export.ExportFile{Content: doc})
	}
	return response
}

// childUnit is a unit under another, naming its parent by id.
func childUnit(id, parent string) string {
	return "resource_type: organization_unit\nid: " + id + "\nname: " + id + "\nparent: " + parent
}

// app is an application in a unit, naming its flow by id.
func app(id, ou, flow string) string {
	return "resource_type: application\nid: " + id + "\nname: " + id + "\nouId: " + ou +
		"\nauthFlowId: \"" + flow + "\"\ncontacts:\n  - admin@example.com\ninboundAuthConfig:\n  - type: oauth2\n" +
		"    config:\n      redirectUris:\n        - https://example.com\n"
}

func flow(id string) string {
	return "resource_type: flow\nid: " + id + "\nname: " + id + "\nnodes:\n  - id: start\n    next: end\n"
}

// What a resource needs is every id of another document its own document names, quoted or not, in a
// mapping or a list; a document's own id is not one.
func TestNeedsAreReadFromTheDocumentsIds(t *testing.T) {
	bundle := parseBundle(joinBundle([]string{unit("ou-1"), childUnit("ou-2", "ou-1"), flow("flow-1"),
		app("app-1", "ou-2", "flow-1"),
		"resource_type: group\nid: grp-1\nname: ou-1\nmembers:\n  - id: usr-1\n    type: user",
		"resource_type: user\nid: usr-1\nouId: ou-1\nattributes:\n  username: ou-2"}))

	needs := needsOf(bundle)

	assert.Empty(t, needs["organization_unit/ou-1"])
	assert.Equal(t, []string{"organization_unit/ou-1"}, needs["organization_unit/ou-2"])
	assert.Equal(t, []string{"flow/flow-1", "organization_unit/ou-2"}, needs["application/app-1"])
	// Any scalar equal to another document's id counts, a name or a username included: ids are UUIDs,
	// so one is never a name by coincidence.
	assert.Equal(t, []string{"organization_unit/ou-1", "user/usr-1"}, needs["group/grp-1"])
	assert.Equal(t, []string{"organization_unit/ou-1", "organization_unit/ou-2"}, needs["user/usr-1"])
}

// A selection is widened to what it needs that the gateway lacks, through what those need in turn;
// a dependency the gateway holds as it is stays out.
func TestASelectionIsWidenedToWhatItNeeds(t *testing.T) {
	changes := []Change{
		{Key: "application/app-1", Change: ChangeAdded, Needs: []string{"flow/flow-1", "organization_unit/ou-2"}},
		{Key: "flow/flow-1", Change: ChangeUnchanged},
		{Key: "organization_unit/ou-2", Change: ChangeAdded, Needs: []string{"organization_unit/ou-1"}},
		{Key: "organization_unit/ou-1", Change: ChangeUpdated},
		{Key: "organization_unit/ou-9", Change: ChangeAdded},
	}

	expanded, required := withNeeded([]string{"application/app-1"}, changes)

	assert.Equal(t, []string{"application/app-1", "organization_unit/ou-2", "organization_unit/ou-1"}, expanded)
	assert.Equal(t, map[string]string{"organization_unit/ou-2": "application/app-1",
		"organization_unit/ou-1": "application/app-1"}, required)

	assert.Equal(t, []string{"application/app-1 needs organization_unit/ou-2"},
		lacking([]string{"application/app-1"}, changes))
	assert.Empty(t, lacking([]string{"application/app-1", "organization_unit/ou-2", "organization_unit/ou-1"}, changes))
}

// A removal is refused while a document the gateway keeps names it, unless that document goes too.
func TestARemovalStillNamedIsRefused(t *testing.T) {
	held := parseBundle(joinBundle([]string{unit("ou-1"), childUnit("ou-2", "ou-1"), flow("flow-1"),
		app("app-1", "ou-2", "flow-1")}))

	assert.Equal(t, []string{"organization_unit/ou-1 is needed by organization_unit/ou-2"},
		stillReferenced([]gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"}}, held))
	assert.Empty(t, stillReferenced([]gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"},
		{ResourceType: "organization_unit", ID: "ou-2"}, {ResourceType: "application", ID: "app-1"}}, held))
	assert.Empty(t, stillReferenced([]gatewayDeletion{{ResourceType: "flow", ID: "flow-9"}}, held))
}

// Deploying one application sends the flow and the unit it needs that the gateway lacks, marked as
// required by it, and reports what each change needs.
func TestDeployingAResourceBringsWhatItNeeds(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), childUnit("ou-2", "ou-1"), flow("flow-1"),
		app("app-1", "ou-2", "flow-1"), unit("ou-9"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"flow/flow-1"}})
	require.Nil(t, svcErr)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"application/app-1"}})

	require.Nil(t, svcErr)
	sent := f.client.last().Content
	assert.Contains(t, sent, "id: app-1")
	assert.Contains(t, sent, "id: ou-2")
	assert.Contains(t, sent, "id: ou-1")
	assert.NotContains(t, sent, "id: flow-1", "a dependency the gateway holds was sent again")
	assert.NotContains(t, sent, "id: ou-9", "a resource nothing selected needs was sent")
	byKey := map[string]Change{}
	for _, change := range result.Diff.Changes {
		byKey[change.Key] = change
	}
	assert.Equal(t, []string{"flow/flow-1", "organization_unit/ou-2"}, byKey["application/app-1"].Needs)
	assert.True(t, byKey["application/app-1"].Selected)
	assert.False(t, byKey["application/app-1"].Required)
	assert.True(t, byKey["organization_unit/ou-1"].Required)
	assert.Equal(t, "application/app-1", byKey["organization_unit/ou-1"].RequiredBy)
	assert.False(t, byKey["organization_unit/ou-9"].Selected)

	// Without dependencies, a selection needing what the gateway lacks is refused before anything is
	// sent, naming what is missing.
	off := false
	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"organization_unit/ou-9"}, WithDependencies: &off})
	require.Nil(t, svcErr, "a resource needing nothing was refused")
	f.exporter.response = documents(unit("ou-1"), childUnit("ou-3", "ou-1"), childUnit("ou-4", "ou-3"))
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	sentBefore := len(f.client.sent)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"organization_unit/ou-4"}, WithDependencies: &off})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorDependencyNotDeployed.Code, svcErr.Code)
	assert.Contains(t, svcErr.ErrorDescription.DefaultValue, "organization_unit/ou-4 needs organization_unit/ou-3")
	assert.Len(t, f.client.sent, sentBefore, "a refused selection reached the gateway")
}

// Removing a resource that a document the gateway keeps still names is refused, with what names it;
// removing both together goes ahead.
func TestRemovingWhatTheGatewayStillNeedsIsRefused(t *testing.T) {
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), childUnit("ou-2", "ou-1"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents()
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"organization_unit/ou-1"}})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalInUse.Code, svcErr.Code)
	assert.Contains(t, svcErr.ErrorDescription.DefaultValue,
		"organization_unit/ou-1 is needed by organization_unit/ou-2")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"organization_unit/ou-1"},
		DryRun: true})
	assert.Equal(t, ErrorRemovalInUse.Code, svcErr.Code, "a dry run was allowed to plan a removal still in use")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Len(t, f.client.last().Deletions, 2)
	assert.True(t, result.Recorded)
}
