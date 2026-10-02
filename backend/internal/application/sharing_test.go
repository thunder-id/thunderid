// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/application/model"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
)

const (
	testAppID    = "m2m-app"
	testAppName  = "Billing Sync"
	testAppOwner = "m2m-root"
	testPolicyID = "m2m-app-share"
)

// ApplicationSharingTestSuite covers how an application participates in sharing: what it declares to
// the framework, and how the policies its file declares are validated and replayed.
type ApplicationSharingTestSuite struct {
	suite.Suite
}

func TestApplicationSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSharingTestSuite))
}

// An application declares no field, which is what makes "a sharee may not edit anything" true by
// construction rather than by a rule somebody has to remember to write.
func (s *ApplicationSharingTestSuite) TestNoFieldsAreDeclared() {
	decl := newApplicationSharingDeclaration(applicationmock.NewApplicationServiceInterfaceMock(s.T()))

	s.Equal(ApplicationSharingType, decl.ResourceType())
	s.Empty(decl.Fields(), "an application shares no per-organization-unit state")
}

// Ownership is read through the runtime context. The framework asks in order to decide access, so an
// access-checked read here would recurse back through the framework.
func (s *ApplicationSharingTestSuite) TestOwnerIsResolvedWithoutAnAccessCheck() {
	appService := applicationmock.NewApplicationServiceInterfaceMock(s.T())
	var sawRuntime bool
	appService.EXPECT().GetApplication(mock.Anything, testAppID).
		RunAndReturn(func(ctx context.Context, _ string) (*providers.Application, *tidcommon.ServiceError) {
			sawRuntime = security.IsRuntimeContext(ctx)
			return &providers.Application{ID: testAppID, OUID: testAppOwner}, nil
		}).Once()

	owner, svcErr := newApplicationSharingDeclaration(appService).OwningOUID(context.Background(), testAppID)

	s.Require().Nil(svcErr)
	s.Equal(testAppOwner, owner)
	s.True(sawRuntime, "the ownership read must not be access-checked")
}

// A failure to read the application is reported rather than answered as "no owner", which the
// framework would read as a resource it does not recognize.
func (s *ApplicationSharingTestSuite) TestOwnerResolutionCarriesTheServiceError() {
	appService := applicationmock.NewApplicationServiceInterfaceMock(s.T())
	appService.EXPECT().GetApplication(mock.Anything, testAppID).
		Return(nil, &tidcommon.InternalServerError).Once()

	owner, svcErr := newApplicationSharingDeclaration(appService).OwningOUID(context.Background(), testAppID)

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Empty(owner)
}

// An application with no sharing framework behind it is usable in its own organization unit alone,
// which is what a deployment that shares nothing should see.
func (s *ApplicationSharingTestSuite) TestVisibilityWithoutTheFrameworkIsFalse() {
	as := &applicationService{}

	visible, svcErr := as.IsApplicationAccessibleFromOU(context.Background(), testAppID, "m2m-child-a")

	s.Require().Nil(svcErr)
	s.False(visible)
}

// The question is passed to the framework as the application resource type, so an application is
// answered by the policies declared for it rather than by anything application-local.
func (s *ApplicationSharingTestSuite) TestVisibilityIsAnsweredByTheFramework() {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingService.EXPECT().
		IsVisible(mock.Anything, ApplicationSharingType, testAppID, "m2m-child-a").
		Return(true, nil).Once()

	visible, svcErr := (&applicationService{sharingService: sharingService}).
		IsApplicationAccessibleFromOU(context.Background(), testAppID, "m2m-child-a")

	s.Require().Nil(svcErr)
	s.True(visible)
}

// The policies have to survive the parse, because the declarative path is the only way in: there is
// no API that could set them afterwards.
func (s *ApplicationSharingTestSuite) TestDeclaredPoliciesSurviveYAMLParsing() {
	doc := []byte(`
id: ` + testAppID + `
name: ` + testAppName + `
ouId: ` + testAppOwner + `
sharingPolicies:
  - id: ` + testPolicyID + `
    targetOuScope:
      allOus: true
      excludedOuIds:
        - m2m-child-b
`)

	var request model.ApplicationRequestWithID
	s.Require().NoError(yaml.Unmarshal(doc, &request))

	s.Require().Len(request.SharingPolicies, 1)
	policy := request.SharingPolicies[0]
	s.Equal(testPolicyID, policy.ID)
	s.True(policy.TargetOuScope.AllOUs)
	s.Equal([]string{"m2m-child-b"}, policy.TargetOuScope.ExcludedOUIDs)
}

// A read made on behalf of an organization unit the application may not act for is refused, and the
// refusal sits in the read so every path reaching an application inherits it.
func (s *ApplicationSharingTestSuite) TestAReadForAnUnreachedOrganizationUnitIsRefused() {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingService.EXPECT().
		IsVisible(mock.Anything, ApplicationSharingType, testAppID, "m2m-child-a").
		Return(false, nil).Once()
	as := &applicationService{sharingService: sharingService}
	ctx := syscontext.WithAccessingOUID(context.Background(), "m2m-child-a")

	svcErr := as.requireAccessibleFromAccessingOU(ctx, testAppID, "m2m-root")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

// The owning organization unit needs no policy, and a read naming none is untouched: an ordinary
// management read must not start consulting the sharing framework.
func (s *ApplicationSharingTestSuite) TestOrdinaryAndOwnerReadsAreUntouched() {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	as := &applicationService{sharingService: sharingService}

	s.Nil(as.requireAccessibleFromAccessingOU(context.Background(), testAppID, "m2m-root"),
		"a read naming no organization unit")
	s.Nil(as.requireAccessibleFromAccessingOU(
		syscontext.WithAccessingOUID(context.Background(), "m2m-root"), testAppID, "m2m-root"),
		"a read naming the owning organization unit")

	sharingService.AssertNotCalled(s.T(), "IsVisible",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// An internal read made while the framework is resolving ownership must not be access-checked, or
// it would recurse back through the framework that asked for it.
func (s *ApplicationSharingTestSuite) TestARuntimeReadIsNotAccessChecked() {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	as := &applicationService{sharingService: sharingService}
	ctx := security.WithRuntimeContext(syscontext.WithAccessingOUID(context.Background(), "m2m-child-a"))

	s.Nil(as.requireAccessibleFromAccessingOU(ctx, testAppID, "m2m-root"))
	sharingService.AssertNotCalled(s.T(), "IsVisible",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A read on behalf of an organization unit a policy reaches goes through, which is the half the
// refusal above is measured against.
func (s *ApplicationSharingTestSuite) TestAReadForAReachedOrganizationUnitIsAllowed() {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingService.EXPECT().
		IsVisible(mock.Anything, ApplicationSharingType, testAppID, "m2m-child-a").
		Return(true, nil).Once()
	as := &applicationService{sharingService: sharingService}
	ctx := syscontext.WithAccessingOUID(context.Background(), "m2m-child-a")

	s.Nil(as.requireAccessibleFromAccessingOU(ctx, testAppID, "m2m-root"))
}

// A framework that cannot answer is not treated as a refusal: its error is carried out as it is, so
// the caller reports why the check failed rather than reporting the application as out of reach.
func (s *ApplicationSharingTestSuite) TestAFrameworkErrorIsCarriedOut() {
	sharingService := sharingmock.NewSharingServiceInterfaceMock(s.T())
	sharingService.EXPECT().
		IsVisible(mock.Anything, ApplicationSharingType, testAppID, "m2m-child-a").
		Return(false, &tidcommon.InternalServerError).Once()
	as := &applicationService{sharingService: sharingService}
	ctx := syscontext.WithAccessingOUID(context.Background(), "m2m-child-a")

	svcErr := as.requireAccessibleFromAccessingOU(ctx, testAppID, "m2m-root")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.NotEqual(tidcommon.ErrorUnauthorized.Code, svcErr.Code,
		"a framework failure must not read as the application being out of reach")
}
