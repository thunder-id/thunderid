// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeProjects is a project directory knowing the given projects, by id and the name they are shown by.
type fakeProjects struct {
	projects map[string]string
	err      *tidcommon.ServiceError
	listErr  *tidcommon.ServiceError
}

func (f *fakeProjects) ProjectExists(_ context.Context, id string) (bool, *tidcommon.ServiceError) {
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.projects[id]
	return ok, nil
}

func (f *fakeProjects) ProjectNames(context.Context) (map[string]string, *tidcommon.ServiceError) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.projects, nil
}

// scopeTarget is the second gateway the suite administers.
const scopeTarget = "gw-2"

// enUS is a translation that belongs to no project.
const enUS = "resource_type: translation\nlanguage: en-US\ntext: hi"

type ProjectScopeTestSuite struct {
	suite.Suite
	f        *versionFixture
	projects *fakeProjects
}

func TestProjectScopeTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectScopeTestSuite))
}

// SetupTest administers gw-1 and gw-2 for a deployment with the projects shop and billing.
func (s *ProjectScopeTestSuite) SetupTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
	// A project's rollback returns to a master before its last deploy, so more than one is kept.
	kept := 10
	config.GetServerRuntime().Config.Gateway.MaxVersions = &kept
	s.f = newVersionFixture(s.T())
	second := s.f.gateways.gateways[0]
	second.ID, second.Name = scopeTarget, testProdName
	s.f.gateways.gateways = append(s.f.gateways.gateways, second)
	s.projects = &fakeProjects{projects: map[string]string{"shop": "Shop", "billing": "billing"}}
	s.f.svc.(*versionService).projects = s.projects
}

func (s *ProjectScopeTestSuite) TearDownTest() {
	config.ResetServerRuntime()
	configureGatewayBound(shippedGatewayBound)
}

// captureDocs captures the given documents as the newest version.
func (s *ProjectScopeTestSuite) captureDocs(docs ...string) {
	s.f.exporter.response = documents(docs...)
	_, svcErr := s.f.svc.Capture(context.Background(), CaptureRequest{})
	s.Require().Nil(svcErr)
}

func sectionsOf(diff *Diff) map[string]string {
	sections := map[string]string{}
	for _, change := range diff.Changes {
		sections[change.Key] = change.Section
	}
	return sections
}

func changeNamed(diff *Diff, key string) Change {
	for _, change := range diff.Changes {
		if change.Key == key {
			return change
		}
	}
	return Change{}
}

// projectApp is an application of a project, in a unit, naming its flow by id.
func projectApp(id, projectID, ou, flow string) string {
	return app(id, ou, flow) + "projectId: " + projectID + "\n"
}

// docs is the suite's configuration: a unit with a team unit below it, holding the shop's application,
// which names flow-1; another unit holding the billing application, which names flow-2; a translation,
// and a user in the first unit.
func docs(shopFlow string) []string {
	return []string{unit("ou-shop"), childUnit("ou-team", "ou-shop"), unit("ou-bill"), shopFlow,
		flow("flow-2"), enUS, projectApp("app-shop", "shop", "ou-team", "flow-1"),
		projectApp("app-bill", "billing", "ou-bill", "flow-2"),
		"resource_type: user\nid: usr-1\nouId: ou-shop\nattributes:\n  username: sam"}
}

// A diff scoped to a project places each change: the resources that name the project, what they need,
// what belongs to no project, and another project's own. A needed change the gateway lacks is required.
func (s *ProjectScopeTestSuite) TestADiffScopedToAProjectPlacesEachChange() {
	s.captureDocs(docs(flow("flow-1"))...)

	diff, svcErr := s.f.svc.Diff(context.Background(), "gw-1", "", "shop")

	s.Require().Nil(svcErr)
	s.Equal(map[string]string{
		"organization_unit/ou-shop": sectionNeeded,
		"organization_unit/ou-team": sectionNeeded,
		"application/app-shop":      sectionProject,
		"flow/flow-1":               sectionNeeded,
		"flow/flow-2":               sectionShared,
		"translation/en-US":         sectionShared,
		"user/usr-1":                sectionShared,
		"organization_unit/ou-bill": sectionShared,
		"application/app-bill":      sectionOther,
	}, sectionsOf(diff))
	s.True(changeNamed(diff, "flow/flow-1").Required)
	s.True(changeNamed(diff, "organization_unit/ou-team").Required)
	s.False(changeNamed(diff, "flow/flow-2").Required)
	s.False(changeNamed(diff, "application/app-shop").Required)

	plain, svcErr := s.f.svc.Diff(context.Background(), "gw-1", "", "")
	s.Require().Nil(svcErr)
	for _, change := range plain.Changes {
		s.Empty(change.Section, "a diff without a project placed %s", change.Key)
	}
}

// A project the deployment does not have is refused, and so is any project on a plane that keeps none.
func (s *ProjectScopeTestSuite) TestAnUnknownProjectIsRefused() {
	s.captureDocs(unit("ou-1"))

	_, svcErr := s.f.svc.Diff(context.Background(), "gw-1", "", "nope")
	s.Equal(&ErrorProjectNotFound, svcErr)
	_, svcErr = s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "nope"})
	s.Equal(&ErrorProjectNotFound, svcErr)
	s.Empty(s.f.client.sent)

	s.f.svc.(*versionService).projects = noProjects{}
	_, svcErr = s.f.svc.Diff(context.Background(), "gw-1", "", "shop")
	s.Equal(&ErrorProjectNotFound, svcErr)

	s.projects.err = &tidcommon.InternalServerError
	s.f.svc.(*versionService).projects = s.projects
	_, svcErr = s.f.svc.Diff(context.Background(), "gw-1", "", "shop")
	s.Equal(&tidcommon.InternalServerError, svcErr)
}

// Without a selection, an apply scoped to a project sends the project's own changes and what they need
// that the gateway lacks, and nothing shared or of another project.
func (s *ProjectScopeTestSuite) TestAnApplyScopedToAProjectSendsItsDefaults() {
	s.captureDocs(docs(flow("flow-1"))...)

	result, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop"})

	s.Require().Nil(svcErr)
	sent := s.f.client.last().Content
	for _, id := range []string{"app-shop", "ou-team", "ou-shop", "flow-1"} {
		s.Contains(sent, "id: "+id+"\n")
	}
	for _, id := range []string{"app-bill", "ou-bill", "flow-2", "usr-1"} {
		s.NotContains(sent, "id: "+id+"\n", "%s was sent with the shop", id)
	}
	s.NotContains(sent, "en-US", "a shared resource nobody selected was sent")
	s.True(changeNamed(&result.Diff, "flow/flow-1").Required)
	s.True(changeNamed(&result.Diff, "flow/flow-1").Selected)
	s.False(changeNamed(&result.Diff, "translation/en-US").Selected)
}

// A selection may add shared changes, but not another project's, and may not leave out what the
// project's resources need that the gateway lacks. The selection is never widened.
func (s *ProjectScopeTestSuite) TestAProjectSelectionIsChecked() {
	s.captureDocs(docs(flow("flow-1"))...)
	own := []string{"application/app-shop", "organization_unit/ou-team", "organization_unit/ou-shop"}

	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop",
		Selection: &[]string{own[0], own[1], own[2], "flow/flow-1", "application/app-bill"}})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorSelectionOfOtherProject.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "application/app-bill")

	_, svcErr = s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop", Selection: &own})
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProjectNeedsLeftOut.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "flow/flow-1")
	s.Empty(s.f.client.sent)

	_, svcErr = s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop",
		Selection: &[]string{own[0], own[1], own[2], "flow/flow-1", "translation/en-US"}})
	s.Require().Nil(svcErr)
	s.Contains(s.f.client.last().Content, "en-US")
}

// A dependency the gateway holds in another form is needed but not required, and names the other
// projects whose resources on the gateway use it.
func (s *ProjectScopeTestSuite) TestANeededChangeTheGatewayHoldsIsOptionalAndNamesItsUsers() {
	held := docs("resource_type: flow\nid: flow-1\nname: old")
	held = append(held, projectApp("app-bill-2", "billing", "ou-bill", "flow-1"))
	s.captureDocs(held...)
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)
	next := docs("resource_type: flow\nid: flow-1\nname: new")
	next[6] += "\ndescription: changed"
	s.captureDocs(append(next, projectApp("app-bill-2", "billing", "ou-bill", "flow-1"))...)

	diff, svcErr := s.f.svc.Diff(context.Background(), "gw-1", "", "shop")

	s.Require().Nil(svcErr)
	flowChange := changeNamed(diff, "flow/flow-1")
	s.Equal(sectionNeeded, flowChange.Section)
	s.False(flowChange.Required)
	s.Equal([]string{"billing"}, flowChange.UsedBy)
	s.Empty(changeNamed(diff, "application/app-shop").UsedBy)

	_, svcErr = s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "shop"})
	s.Require().Nil(svcErr)
	sent := s.f.client.last().Content
	s.Contains(sent, "id: app-shop")
	s.NotContains(sent, "name: new", "a held dependency was replaced unasked")
}

// A promotion scoped to a project sends what the source holds of the project's defaults, or of the
// selection given.
func (s *ProjectScopeTestSuite) TestAPromotionScopedToAProjectSendsWhatTheSourceHolds() {
	s.captureDocs(docs(flow("flow-1"))...)
	_, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{})
	s.Require().Nil(svcErr)

	_, svcErr = s.f.svc.Promote(context.Background(), PromoteRequest{SourceGatewayID: "gw-1",
		TargetGatewayID: scopeTarget, Project: "shop"})

	s.Require().Nil(svcErr)
	sent := s.f.client.last().Content
	s.Contains(sent, "id: app-shop")
	s.Contains(sent, "id: flow-1")
	s.NotContains(sent, "id: app-bill")
	s.NotContains(sent, "en-US")

	_, svcErr = s.f.svc.Promote(context.Background(), PromoteRequest{SourceGatewayID: "gw-1",
		TargetGatewayID: scopeTarget, Project: "shop", Selection: &[]string{"translation/en-US"}})
	s.Require().Nil(svcErr)
	s.Equal(enUS, s.f.client.last().Content)
}

// Ownership is read from the documents: a project's own document, and a resource that names a project.
// Units, users and groups name none, and a later document decides for a resource both sets have.
func TestOwnersAreReadFromTheDocuments(t *testing.T) {
	held := parseBundle(joinBundle([]string{
		"resource_type: project\nid: shop\nhandle: shop",
		unit("ou-shop"),
		"resource_type: role\nid: r1\nname: r1\nouId: ou-shop",
		"resource_type: agent\nid: ag-1\nname: ag\nprojectId: billing",
		"resource_type: agent\nid: ag-2\nname: ag2\nprojectId: shop",
		"resource_type: group\nid: g1\nname: g\nouId: ou-shop",
		"resource_type: user\nid: u1\nouId: ou-shop",
	}))
	version := parseBundle(joinBundle([]string{"resource_type: agent\nid: ag-2\nname: ag2"}))

	owners := ownersOf(held, version)

	assert.Equal(t, map[string]string{
		"project/shop": "shop",
		"agent/ag-1":   "billing",
	}, owners)
}

// A project is shown by its id when the projects cannot be named.
func TestUsedByFallsBackToTheProjectID(t *testing.T) {
	held := parseBundle(joinBundle([]string{flow("flow-1"), app("a", "ou-1", "flow-1")}))
	diff := &Diff{Changes: []Change{{Key: "flow/flow-1", ID: "flow-1", Change: ChangeUpdated,
		Section: sectionShared}}}

	projectScope{projectID: "shop", owners: map[string]string{"application/a": "ghost"}}.markUsedBy(
		diff, held, map[string]string{})

	assert.Equal(t, []string{"ghost"}, diff.Changes[0].UsedBy)
}

// A promotion that may send nothing of the project sends nothing.
func TestAScopedSelectionNarrowedToNothingIsEmpty(t *testing.T) {
	scope := projectScope{projectID: "shop", defaults: []string{"application/a"},
		required: map[string]bool{}, others: map[string]bool{}, owners: map[string]string{}}

	selection, svcErr := scopedSelection(scope, nil, &[]string{})

	require.Nil(t, svcErr)
	assert.Equal(t, []string{}, selection)
}

// A refusal names one resource in the singular and several in the plural.
func TestRefusalsAgreeWithTheirCount(t *testing.T) {
	scope := projectScope{projectID: "shop", others: map[string]bool{"application/a": true, "application/b": true},
		required: map[string]bool{"flow/f": true}, owners: map[string]string{"application/s": "shop"},
		needs: map[string][]string{"application/s": {"flow/f"}}}

	one := scope.check([]string{"application/a"})
	several := scope.check([]string{"application/a", "application/b"})
	missing := scope.check([]string{"application/s"})

	assert.Contains(t, one.ErrorDescription.DefaultValue, "application/a belongs to another project, which deploys it")
	assert.Contains(t, several.ErrorDescription.DefaultValue, "belong to other projects, which deploy them")
	assert.Contains(t, missing.ErrorDescription.DefaultValue, "flow/f, which the gateway does not hold. Select it")
}

// What another project owns and the gateway holds is that project's, even when the project's resources name
// it, and what it names in turn is not followed: it is never placed as needed, and so never sent with this
// project.
func TestReachStopsAtAnotherProjectsResourcesTheGatewayHolds(t *testing.T) {
	diff := &Diff{Changes: []Change{
		{Key: "application/shop", Change: ChangeAdded, Needs: []string{"application/bill"}},
		{Key: "application/bill", Change: ChangeUpdated, Needs: []string{"flow/bill-only"}},
		{Key: "flow/bill-only", Change: ChangeAdded},
	}}
	owners := map[string]string{"application/shop": "shop", "application/bill": "billing"}

	scope := scopeToProject("shop", owners, diff)

	assert.Equal(t, map[string]string{"application/shop": sectionProject, "application/bill": sectionOther,
		"flow/bill-only": sectionShared}, sectionsOf(diff))
	assert.Equal(t, []string{"application/shop"}, scope.defaults)
	assert.False(t, changeNamed(diff, "flow/bill-only").Required)
}

// Another project's resource the project's resources need and the gateway does not hold is needed, with what
// it needs in turn, so the project deploys before that project has: both are required and sent by default.
func TestAnotherProjectsResourceTheGatewayLacksIsBroughtAlong(t *testing.T) {
	diff := &Diff{Changes: []Change{
		{Key: "application/shop", Change: ChangeAdded, Needs: []string{"flow/login"}},
		{Key: "flow/login", Change: ChangeAdded, Needs: []string{"flow/bill-only"}},
		{Key: "flow/bill-only", Change: ChangeAdded},
	}}
	owners := map[string]string{"application/shop": "shop", "flow/login": "default", "flow/bill-only": "default"}

	scope := scopeToProject("shop", owners, diff)

	assert.Equal(t, map[string]string{"application/shop": sectionProject, "flow/login": sectionNeeded,
		"flow/bill-only": sectionNeeded}, sectionsOf(diff))
	assert.True(t, changeNamed(diff, "flow/login").Required)
	assert.ElementsMatch(t, []string{"application/shop", "flow/login", "flow/bill-only"}, scope.defaults)
	assert.Nil(t, scope.check(scope.defaults))
}

// A needed change the gateway holds back stays required but is not sent by default: only selecting it
// stops holding it back.
func TestAHeldBackRequiredChangeIsNotADefault(t *testing.T) {
	diff := &Diff{Changes: []Change{
		{Key: "application/shop", Change: ChangeAdded, Needs: []string{"flow/f"}},
		{Key: "flow/f", Change: ChangeAdded, HeldBack: true},
	}}

	scope := scopeToProject("shop", map[string]string{"application/shop": "shop"}, diff)

	assert.True(t, changeNamed(diff, "flow/f").Required)
	assert.Equal(t, []string{"application/shop"}, scope.defaults)
	assert.NotNil(t, scope.check(scope.defaults), "a default apply left out a held-back required change silently")
}

// A project deploys to a gateway the default project has not been deployed to: the organization's unit and
// the project's application name the sign-in flow the default project holds, which the deploy brings along.
func (s *ProjectScopeTestSuite) TestAProjectDeploysWithoutTheDefaultProjectBeforeIt() {
	s.projects.projects["default"] = "Default"
	s.captureDocs(
		"resource_type: organization_unit\nid: ou-default\nname: default\nauthFlowId: flow-login",
		"resource_type: flow\nid: flow-login\nname: login\nprojectId: default",
		projectApp("app-bill", "billing", "ou-default", "flow-login"),
	)

	result, svcErr := s.f.svc.Apply(context.Background(), "gw-1", ApplyRequest{Project: "billing"})

	s.Require().Nil(svcErr, "%v", svcErr)
	sent := s.f.client.last().Content
	s.Contains(sent, "id: app-bill")
	s.Contains(sent, "id: ou-default")
	s.Contains(sent, "id: flow-login", "the flow the project needs was not brought along")
	s.NotNil(result)
}
