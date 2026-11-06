// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
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
	// A value under a field that describes the resource, such as a name or a username, is never a
	// reference, even when it equals another document's id.
	assert.Equal(t, []string{"user/usr-1"}, needs["group/grp-1"])
	assert.Equal(t, []string{"organization_unit/ou-1"}, needs["user/usr-1"])
}

// Resources of different types that share an id are all needed by a document naming it, since the
// document names the id alone.
func (s *DependenciesTestSuite) TestAnIDSharedAcrossTypesNeedsEach() {
	needs := needsOf(parseBundle(joinBundle([]string{unit("same"), flow("same"),
		"resource_type: role\nid: r-1\nname: r\nouId: same"})))

	s.Equal([]string{"flow/same", "organization_unit/same"}, needs["role/r-1"])
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

	merged, ok := mergeAnswers(
		json.RawMessage(`{"summary":{"imported":2,"failed":0},"results":[{"status":"success"}]}`),
		json.RawMessage(`{"summary":{"deleted":1,"failed":0},"results":[{"operation":"delete","status":"success"}]}`))
	assert.True(t, ok)
	assert.JSONEq(t, `{"summary":{"imported":2,"deleted":1,"failed":0},"results":[{"status":"success"},`+
		`{"operation":"delete","status":"success"}]}`, string(merged))
}

// A removal that a waiting removal's document names waits with it: the gateway keeps that document until
// the waiting removal is made, so removing what it names first would be refused.
func (s *DependenciesTestSuite) TestADeferralFollowsTheRemovalsItHolds() {
	held := parseBundle(joinBundle([]string{
		"resource_type: role\nid: y\nname: y\nouId: x",
		"resource_type: organization_unit\nid: x\nname: x\nparent: p",
		unit("p"),
	}))
	updated := parseBundle("resource_type: role\nid: y\nname: y\nouId: other")
	deletions := []gatewayDeletion{{ResourceType: "organization_unit", ID: "x"},
		{ResourceType: "organization_unit", ID: "p"}}

	now, after, awaiting := deferredRemovals(deletions, held, updated)

	s.Empty(now)
	s.Equal(deletions, after, "the order the removals were given in changed")
	s.Equal(awaiting["organization_unit/x"], awaiting["organization_unit/p"])
}

// What the first import wrote is recorded even when the second, for the removals the update freed,
// fails or answers unreadably; those removals stay pending, offered again by the diff.
func (s *DependenciesTestSuite) TestTheFirstImportIsRecordedWhenTheSecondFails() {
	for name, second := range map[string]func(f *fakeGatewayClient){
		"fails":      func(f *fakeGatewayClient) { f.err = errors.New("connection reset") },
		"unreadable": func(f *fakeGatewayClient) { f.imported = json.RawMessage(`{"results":`) },
	} {
		s.Run(name, func() {
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
			f.client.onImport = func() {
				if len(f.client.sent) > 0 && f.client.last().Content == "" {
					second(f.client)
				}
			}

			_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})

			require.Nil(t, svcErr)
			master, svcErr := f.svc.GetMaster(context.Background(), "gw-1")
			require.Nil(t, svcErr)
			assert.Contains(t, master.Resources, "parent: ou-3", "the first import's update was not recorded")
			diff, svcErr := f.svc.Diff(context.Background(), "gw-1", "")
			require.Nil(t, svcErr)
			offered := false
			for _, change := range diff.Changes {
				offered = offered || (change.Key == "organization_unit/ou-1" && change.Change == ChangeDeleted)
			}
			assert.True(t, offered, "the unconfirmed removal is no longer offered")
		})
	}
	_, ok := mergeAnswers(json.RawMessage(`{"results":[]}`), json.RawMessage(`not json`))
	s.False(ok)
}

// A removal that two updates free, one directly and one through another removal, waits for both: it is
// made only once every update it depends on was accepted.
func (s *DependenciesTestSuite) TestAConvergingDeferralWaitsForEveryUpdate() {
	held := parseBundle(joinBundle([]string{
		"resource_type: role\nid: y1\nname: y1\nouId: x",
		"resource_type: role\nid: y2\nname: y2\nouId: p",
		"resource_type: organization_unit\nid: x\nname: x\nparent: p",
		unit("p"),
	}))
	updated := parseBundle(joinBundle([]string{"resource_type: role\nid: y1\nname: y1\nouId: o",
		"resource_type: role\nid: y2\nname: y2\nouId: o"}))
	deletions := []gatewayDeletion{{ResourceType: "organization_unit", ID: "x"},
		{ResourceType: "organization_unit", ID: "p"}}

	_, after, awaiting := deferredRemovals(deletions, held, updated)

	s.Equal(deletions, after)
	keys := make([]string, 0, len(awaiting["organization_unit/p"]))
	for _, resource := range awaiting["organization_unit/p"] {
		keys = append(keys, resource.key())
	}
	s.ElementsMatch([]string{"role/y1", "role/y2"}, keys)
	released := releasedRemovals(after, awaiting, answerOf(
		importOutcome{ResourceType: "role", ResourceID: "y1", Status: outcomeFailed, Code: "R-1"},
		importOutcome{ResourceType: "role", ResourceID: "y2", Status: outcomeSuccess}))
	s.Empty(released, "a removal was made while an update it waits for was refused")
}

// A second answer without results, or with null results, is not one that says how the removals went.
func (s *DependenciesTestSuite) TestASecondAnswerWithoutResultsIsUnread() {
	first := json.RawMessage(`{"summary":{"imported":1},"results":[]}`)
	for _, second := range []string{`{"summary":{"deleted":1}}`, `{"summary":{"deleted":1},"results":null}`} {
		merged, ok := mergeAnswers(first, json.RawMessage(second))
		s.False(ok, second)
		s.JSONEq(string(first), string(merged))
	}
}

// A document's own id and kind are not references: two resources sharing an id do not need each other,
// and a kind such as `type: user` does not need a resource whose id is `user`. A nested id still is one.
func (s *DependenciesTestSuite) TestADocumentsOwnIdentityIsNotAReference() {
	needs := needsOf(parseBundle(joinBundle([]string{unit("same"), flow("same"), unit("user"),
		"resource_type: user\nid: u-1\ntype: user\nouId: o",
		"resource_type: group\nid: g-1\nname: g\nmembers:\n  - id: u-1\n    type: user"})))

	s.Empty(needs["organization_unit/same"])
	s.Empty(needs["flow/same"])
	s.Empty(needs["user/u-1"], "a kind was read as a reference")
	s.Equal([]string{"user/u-1"}, needs["group/g-1"])
}
