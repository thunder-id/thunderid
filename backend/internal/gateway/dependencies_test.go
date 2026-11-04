// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/export"
)

// DependenciesTestSuite covers what a resource needs, read from the version, and how an apply brings it
// along or refuses to take it away. Every test starts from the package's shipped configuration.
type DependenciesTestSuite struct {
	suite.Suite
}

func TestDependenciesTestSuite(t *testing.T) {
	suite.Run(t, new(DependenciesTestSuite))
}

func (s *DependenciesTestSuite) SetupTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

func (s *DependenciesTestSuite) TearDownTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

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
func (s *DependenciesTestSuite) TestNeedsAreReadFromTheDocumentsIds() {
	t := s.T()
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
func (s *DependenciesTestSuite) TestASelectionIsWidenedToWhatItNeeds() {
	t := s.T()
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

	missing, heldBack := lacking([]string{"application/app-1"}, changes)
	assert.Equal(t, []string{"application/app-1 needs organization_unit/ou-2"}, missing)
	assert.False(t, heldBack)
	missing, _ = lacking([]string{"application/app-1", "organization_unit/ou-2", "organization_unit/ou-1"}, changes)
	assert.Empty(t, missing)

	changes[2].HeldBack = true
	_, heldBack = lacking([]string{"application/app-1"}, changes)
	assert.True(t, heldBack, "a held-back dependency was not reported as held back")
}

// A selection sent without its dependencies is refused naming what it lacks, as a parameter so a
// translation keeps it; when the gateway holds some of that back, it is not told to send dependencies
// along, which would not take it in.
func (s *DependenciesTestSuite) TestARefusalNamesWhatIsMissing() {
	t := s.T()
	missing := []string{"application/app-1 needs organization_unit/ou-2"}

	refused := dependenciesMissing(missing, false)
	assert.Equal(t, ErrorDependencyNotDeployed.Code, refused.Code)
	assert.Equal(t, "error.gatewayservice.dependency_missing.description", refused.ErrorDescription.Key)
	assert.Equal(t, missing[0], refused.ErrorDescription.Params["missing"])
	assert.Contains(t, refused.ErrorDescription.String(), "withDependencies")

	refused = dependenciesMissing(missing, true)
	assert.Equal(t, "error.gatewayservice.dependency_held_back_missing.description", refused.ErrorDescription.Key)
	assert.NotContains(t, refused.ErrorDescription.String(), "withDependencies")
	assert.Contains(t, refused.ErrorDescription.String(), missing[0])
}

// A removal goes before the removals of what its document names, through what those name in turn;
// removals that need nothing of each other keep their order.
func (s *DependenciesTestSuite) TestRemovalsGoBeforeWhatTheyNeed() {
	t := s.T()
	held := parseBundle(joinBundle([]string{unit("ou-1"), childUnit("ou-2", "ou-1"), childUnit("ou-3", "ou-2"),
		unit("ou-9")}))
	removal := func(id string) gatewayDeletion { return gatewayDeletion{ResourceType: "organization_unit", ID: id} }

	ordered := orderedRemovals([]gatewayDeletion{removal("ou-1"), removal("ou-9"), removal("ou-2"),
		removal("ou-3")}, held)

	assert.Equal(t, []gatewayDeletion{removal("ou-3"), removal("ou-2"), removal("ou-1"), removal("ou-9")}, ordered)
	assert.Equal(t, []gatewayDeletion{removal("ou-1")}, orderedRemovals([]gatewayDeletion{removal("ou-1")}, held))
}

// Sent without its dependencies, a selection lacks only what the gateway does not hold or refused: a
// dependency the gateway holds in another form serves the selected change, and replacing it is left
// to be chosen on its own.
func (s *DependenciesTestSuite) TestADependencyHeldInAnotherFormIsNotLacking() {
	t := s.T()
	changes := []Change{
		{Key: "application/app-1", Change: ChangeAdded,
			Needs: []string{"flow/flow-1", "organization_unit/ou-1", "organization_unit/ou-2"}},
		{Key: "flow/flow-1", Change: ChangeUpdated},
		{Key: "organization_unit/ou-1", Change: ChangeUpdated, Failed: true},
		{Key: "organization_unit/ou-2", Change: ChangeAdded},
	}

	missing, heldBack := lacking([]string{"application/app-1"}, changes)
	assert.Equal(t, []string{"application/app-1 needs organization_unit/ou-1",
		"application/app-1 needs organization_unit/ou-2"}, missing)
	assert.False(t, heldBack)
}

// A removal is refused while a document the gateway keeps names it, unless that document goes too.
func (s *DependenciesTestSuite) TestARemovalStillNamedIsRefused() {
	t := s.T()
	held := parseBundle(joinBundle([]string{unit("ou-1"), childUnit("ou-2", "ou-1"), flow("flow-1"),
		app("app-1", "ou-2", "flow-1")}))

	assert.Equal(t, []string{"organization_unit/ou-1 is needed by organization_unit/ou-2"},
		stillReferenced([]gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"}}, held, nil))
	assert.Empty(t, stillReferenced([]gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"},
		{ResourceType: "organization_unit", ID: "ou-2"}, {ResourceType: "application", ID: "app-1"}}, held, nil))
	assert.Empty(t, stillReferenced([]gatewayDeletion{{ResourceType: "flow", ID: "flow-9"}}, held, nil))
}

// Deploying one application sends the flow and the unit it needs that the gateway lacks, marked as
// required by it, and reports what each change needs.
func (s *DependenciesTestSuite) TestDeployingAResourceBringsWhatItNeeds() {
	t := s.T()
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
	assert.Contains(t, svcErr.ErrorDescription.String(), "organization_unit/ou-4 needs organization_unit/ou-3")
	assert.Len(t, f.client.sent, sentBefore, "a refused selection reached the gateway")
}

// A dependency the gateway holds back is not taken in for a selection that needs it: the apply is
// refused naming it, and it stays held back until a selection names it.
func (s *DependenciesTestSuite) TestADependencyHeldBackIsNotSentForWhatNeedsIt() {
	t := s.T()
	changes := []Change{
		{Key: "application/app-2", Change: ChangeAdded, Needs: []string{"flow/flow-2"}},
		{Key: "flow/flow-2", Change: ChangeAdded, HeldBack: true},
	}
	expanded, required := withNeeded([]string{"application/app-2"}, changes)
	assert.Equal(t, []string{"application/app-2"}, expanded)
	assert.Empty(t, required)

	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), flow("flow-2"), app("app-2", "ou-1", "flow-2"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{
		Selection: &[]string{"organization_unit/ou-1"}, LeaveOut: []string{"flow/flow-2"}})
	require.Nil(t, svcErr)
	sentBefore := len(f.client.sent)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"application/app-2"}})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorDependencyNotDeployed.Code, svcErr.Code)
	assert.Contains(t, svcErr.ErrorDescription.String(), "application/app-2 needs flow/flow-2")
	assert.Contains(t, svcErr.ErrorDescription.DefaultValue, "holds that back")
	assert.Len(t, f.client.sent, sentBefore, "a refused selection reached the gateway")
	assert.Len(t, f.versions.excluded["gw-1"], 1, "the dependency stopped being held back")

	// An apply without a selection leaves it out too, so what needs it is refused the same way.
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorDependencyNotDeployed.Code, svcErr.Code)

	// Selecting both sends both and releases the dependency.
	_, svcErr = f.svc.Apply(context.Background(), "gw-1",
		ApplyRequest{Selection: &[]string{"application/app-2", "flow/flow-2"}})
	require.Nil(t, svcErr)
	assert.Contains(t, f.client.last().Content, "id: flow-2")
	assert.Contains(t, f.client.last().Content, "id: app-2")
	assert.Empty(t, f.versions.excluded["gw-1"])
}

// Removing a resource that a document the gateway keeps still names is refused, with what names it;
// removing both together goes ahead.
func (s *DependenciesTestSuite) TestRemovingWhatTheGatewayStillNeedsIsRefused() {
	t := s.T()
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
	assert.Contains(t, svcErr.ErrorDescription.String(),
		"organization_unit/ou-1 is needed by organization_unit/ou-2")
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Selection: &[]string{"organization_unit/ou-1"},
		DryRun: true})
	assert.Equal(t, ErrorRemovalInUse.Code, svcErr.Code, "a dry run was allowed to plan a removal still in use")

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	assert.Len(t, f.client.last().Deletions, 2)
	assert.True(t, result.Recorded)
}

// A removal stays refused while the document that names it is updated away in the same apply, since
// the gateway can refuse the update and still make the removal; once the update is deployed, the
// removal goes ahead.
func (s *DependenciesTestSuite) TestARemovalGoesAfterTheUpdateThatFreesIt() {
	t := s.T()
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), unit("ou-3"), childUnit("ou-2", "ou-1"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents(unit("ou-3"), childUnit("ou-2", "ou-3"))
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	sentBefore := len(f.client.sent)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	require.Len(t, f.client.sent, sentBefore+2, "the removal was not sent on its own after the update")
	first, second := f.client.sent[sentBefore], f.client.sent[sentBefore+1]
	assert.Equal(t, childUnit("ou-2", "ou-3"), first.Content)
	assert.Empty(t, first.Deletions, "the removal was sent with the update that frees it")
	assert.Empty(t, second.Content)
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"}}, second.Deletions)
	assert.True(t, result.Recorded)
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	removed, _ := entryOf(master, "organization_unit/ou-1")
	assert.Equal(t, EntryRemoved, removed.State)
	assert.Equal(t, MasterSummary{Deployed: 2, Removed: 1}, master.Summary)
}

// When the gateway refuses the update that would free a removal, the removal is not sent: the resource
// stays in place, recorded as the gateway keeps it, and the diff offers the removal again.
func (s *DependenciesTestSuite) TestARemovalWaitsWhileItsUpdateIsRefused() {
	t := s.T()
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), childUnit("ou-2", "ou-1"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents(unit("ou-3"), childUnit("ou-2", "ou-3"))
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	f.client.imported = answerOf(wrote("ou-3"), refused("ou-2", "ou-2", "OU-7"))
	sentBefore := len(f.client.sent)

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	require.Nil(t, svcErr)
	assert.Len(t, f.client.sent, sentBefore+1, "a removal was sent although its update was refused")
	assert.Empty(t, f.client.last().Deletions)
	assert.True(t, result.Recorded)
	master, _ := f.svc.GetMaster(context.Background(), "gw-1")
	kept, _ := entryOf(master, "organization_unit/ou-1")
	assert.Equal(t, EntryDeployed, kept.State, "the resource the refused update still names was dropped")
	diff, _ := f.svc.Diff(context.Background(), "gw-1", "")
	offered := map[string]ChangeKind{}
	for _, change := range diff.Changes {
		offered[change.Key] = change.Change
	}
	assert.Equal(t, ChangeDeleted, offered["organization_unit/ou-1"])
	assert.Equal(t, ChangeUpdated, offered["organization_unit/ou-2"])
}

// When another node takes the gateway over after the update, the removal the update freed is not sent:
// the resource stays in place for a later apply.
func (s *DependenciesTestSuite) TestAFreedRemovalIsNotSentOnceTheLeaseIsLost() {
	t := s.T()
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), unit("ou-3"), childUnit("ou-2", "ou-1"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents(unit("ou-3"), childUnit("ou-2", "ou-3"))
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	sentBefore := len(f.client.sent)
	f.client.onImport = func() { f.versions.leases["gw-1"].Token = anotherPod }

	_, _ = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	assert.Len(t, f.client.sent, sentBefore+1, "a removal was sent after the lease was lost")
	assert.Empty(t, f.client.last().Deletions)
}

// An apply that would be queued behind another is refused at once for a removal that what the gateway
// keeps still needs, as the queued run would refuse it.
func (s *DependenciesTestSuite) TestARemovalInUseIsRefusedBeforeQueueing() {
	t := s.T()
	shortLeaseWait(t)
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), childUnit("ou-2", "ou-1"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents(childUnit("ou-2", "ou-1"))
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	f.versions.leases["gw-1"].Token, f.versions.leases["gw-1"].Since = nodeElsewhere, time.Now()

	result, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

	assert.Nil(t, result)
	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorRemovalInUse.Code, svcErr.Code)
	assert.Contains(t, svcErr.ErrorDescription.String(), "organization_unit/ou-1 is needed by organization_unit/ou-2")
}

// A dry run sends every removal at once, since nothing is written, and two answers merge into one.
func (s *DependenciesTestSuite) TestADryRunSendsEveryRemovalAtOnce() {
	t := s.T()
	f := newVersionFixture(t)
	f.exporter.response = documents(unit("ou-1"), childUnit("ou-2", "ou-1"))
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	require.Nil(t, svcErr)
	f.exporter.response = documents(unit("ou-3"), childUnit("ou-2", "ou-3"))
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(t, svcErr)

	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})

	require.Nil(t, svcErr)
	assert.True(t, f.client.last().DryRun)
	assert.Equal(t, []gatewayDeletion{{ResourceType: "organization_unit", ID: "ou-1"}}, f.client.last().Deletions)

	merged := mergeAnswers(json.RawMessage(`{"summary":{"imported":2,"failed":0},"results":[{"status":"success"}]}`),
		json.RawMessage(`{"summary":{"deleted":1,"failed":0},"results":[{"operation":"delete","status":"success"}]}`))
	assert.JSONEq(t, `{"summary":{"imported":2,"deleted":1,"failed":0},"results":[{"status":"success"},`+
		`{"operation":"delete","status":"success"}]}`, string(merged))
}
