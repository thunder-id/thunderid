// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"errors"
	"strings"

	"github.com/stretchr/testify/mock"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testProjectID      = "project-1"
	testOtherProjectID = "project-2"
)

// servesProjects makes the service act for a deployment that serves projects, where every type belongs to one.
func (s *ServiceTestSuite) servesProjects() *projectCheckerMock {
	projects := newProjectCheckerMock(s.T())
	s.service.projects = projects
	s.service.projectRequired = true
	return projects
}

// inProject returns a type as the management API passes it, in the given project.
func inProject(t providers.AuthorizationDetailType, projectID string) providers.AuthorizationDetailType {
	t = apiType(t)
	t.ProjectID = projectID
	return t
}

// A deployment that serves projects refuses a type without one, or in a project that does not exist, and creates
// one in a project that exists.
func (s *ServiceTestSuite) TestCreateType_InAProject() {
	projects := s.servesProjects()

	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, apiType(flightType()))
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProjectRequired.Code, svcErr.Code)

	projects.EXPECT().ProjectExists(mock.Anything, "missing").Return(false, nil).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, inProject(flightType(), "missing"))
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidProject.Code, svcErr.Code)

	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, inProject(flightType(), strings.Repeat("p", 37)))
	s.Require().NotNil(svcErr, "a project ID longer than the column names no project")
	s.Equal(ErrorInvalidProject.Code, svcErr.Code)

	projects.EXPECT().ProjectExists(mock.Anything, testProjectID).
		Return(false, &tidcommon.InternalServerError).Once()
	_, svcErr = s.service.CreateAuthorizationDetailType(s.ctx, inProject(flightType(), testProjectID))
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)

	projects.EXPECT().ProjectExists(mock.Anything, testProjectID).Return(true, nil).Once()
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
		return t.ProjectID == testProjectID
	})).Return(nil).Once()
	created, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, inProject(flightType(), testProjectID))
	s.Require().Nil(svcErr)
	s.Equal(testProjectID, created.ProjectID)
}

// A deployment that serves no projects, such as a data plane, takes a type without a project, and keeps the
// project of one it is given unchecked.
func (s *ServiceTestSuite) TestCreateType_WithoutProjects() {
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Twice()
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.Anything).Return(nil).Twice()

	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, apiType(flightType()))
	s.Nil(svcErr)
	created, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, inProject(flightType(), testProjectID))
	s.Require().Nil(svcErr)
	s.Equal(testProjectID, created.ProjectID)
}

// An update that names no project keeps the type's project without checking it, and one that names another
// project moves the type there once the project is found.
func (s *ServiceTestSuite) TestUpdateType_KeepsOrMovesTheProject() {
	projects := s.servesProjects()
	stored := flightType()
	stored.ProjectID = testProjectID

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	kept, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", apiType(flightType()))
	s.Require().Nil(svcErr)
	s.Equal(testProjectID, kept.ProjectID, "an unchanged type keeps its project and is not saved")

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	projects.EXPECT().ProjectExists(mock.Anything, "missing").Return(false, nil).Once()
	_, svcErr = s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", inProject(flightType(), "missing"))
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidProject.Code, svcErr.Code)

	moved := stored
	moved.ProjectID = testOtherProjectID
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	projects.EXPECT().ProjectExists(mock.Anything, testOtherProjectID).Return(true, nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.MatchedBy(func(t providers.AuthorizationDetailType) bool {
		return t.ProjectID == testOtherProjectID
	}), false).Return(nil).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(moved, nil).Once()
	got, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", inProject(flightType(), testOtherProjectID))
	s.Require().Nil(svcErr)
	s.Equal(testOtherProjectID, got.ProjectID, "a move to another project keeps the version, which approvals rely on")
}

// A move to another project is refused when the project cannot be checked.
func (s *ServiceTestSuite) TestUpdateType_FailsWhenTheProjectCannotBeChecked() {
	projects := s.servesProjects()
	stored := flightType()
	stored.ProjectID = testProjectID
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	projects.EXPECT().ProjectExists(mock.Anything, testOtherProjectID).
		Return(false, &tidcommon.InternalServerError).Once()

	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, "type-1", inProject(flightType(), testOtherProjectID))

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// Validation, as an import's dry run makes it, checks the project as the write would: a new type needs a project
// that exists, and a stored type is checked only when it moves to another project.
func (s *ServiceTestSuite) TestValidateType_ChecksTheProject() {
	projects := s.servesProjects()

	svcErr := s.service.ValidateAuthorizationDetailType(s.ctx, apiType(flightType()))
	s.Require().NotNil(svcErr)
	s.Equal(ErrorProjectRequired.Code, svcErr.Code)

	projects.EXPECT().ProjectExists(mock.Anything, "missing").Return(false, nil).Once()
	svcErr = s.service.ValidateAuthorizationDetailType(s.ctx, inProject(flightType(), "missing"))
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidProject.Code, svcErr.Code)

	stored := flightType()
	stored.ProjectID = testProjectID
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	s.Nil(s.service.ValidateAuthorizationDetailType(s.ctx, flightType()), "an update without a project keeps it")

	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(stored, nil).Once()
	projects.EXPECT().ProjectExists(mock.Anything, testOtherProjectID).Return(false, nil).Once()
	moved := flightType()
	moved.ProjectID = testOtherProjectID
	svcErr = s.service.ValidateAuthorizationDetailType(s.ctx, moved)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidProject.Code, svcErr.Code)
}

// Validation checks the project last, after the processor credential, so a dry-run import that takes a project
// the same import creates as there has checked everything else.
func (s *ServiceTestSuite) TestValidateType_ChecksTheProjectLast() {
	withTestCrypto(s.T())
	s.servesProjects()
	masked, maskErr := sentAuthenticationProperties(bearer("******"))
	s.Require().Nil(maskErr)
	t := inProject(paymentType(), "missing")
	t.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL, AuthenticationProperties: masked}

	svcErr := s.service.ValidateAuthorizationDetailType(s.ctx, t)

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidProcessorAuthentication.Code, svcErr.Code)
}

// A project's types are listed a page at a time, and an empty project ID lists none rather than every type.
func (s *ServiceTestSuite) TestListTypesByProject() {
	s.typeStore.EXPECT().CountTypesByProject(mock.Anything, testProjectID).Return(1, nil).Once()
	s.typeStore.EXPECT().ListTypePageByProject(mock.Anything, testProjectID, 30, 0).
		Return([]providers.AuthorizationDetailType{flightType()}, nil).Once()
	page, svcErr := s.service.ListAuthorizationDetailTypesByProject(s.ctx, testProjectID, 30, 0)
	s.Require().Nil(svcErr)
	s.Equal(1, page.TotalResults)
	s.Len(page.Types, 1)

	page, svcErr = s.service.ListAuthorizationDetailTypesByProject(s.ctx, "", 30, 0)
	s.Require().Nil(svcErr)
	s.Zero(page.TotalResults)
	s.Empty(page.Types)

	_, svcErr = s.service.ListAuthorizationDetailTypesByProject(s.ctx, testProjectID, 0, 0)
	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidPagination.Code, svcErr.Code)

	s.typeStore.EXPECT().CountTypesByProject(mock.Anything, testProjectID).Return(0, errors.New("db down")).Once()
	_, svcErr = s.service.ListAuthorizationDetailTypesByProject(s.ctx, testProjectID, 30, 0)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// A project that still has a type is reported as holding one, which keeps it from being deleted.
func (s *ServiceTestSuite) TestProjectHasAuthorizationDetailTypes() {
	s.typeStore.EXPECT().CountTypesByProject(mock.Anything, testProjectID).Return(2, nil).Once()
	has, svcErr := s.service.ProjectHasAuthorizationDetailTypes(s.ctx, testProjectID)
	s.Nil(svcErr)
	s.True(has)

	s.typeStore.EXPECT().CountTypesByProject(mock.Anything, testOtherProjectID).Return(0, nil).Once()
	has, svcErr = s.service.ProjectHasAuthorizationDetailTypes(s.ctx, testOtherProjectID)
	s.Nil(svcErr)
	s.False(has)

	s.typeStore.EXPECT().CountTypesByProject(mock.Anything, "project-3").Return(0, errors.New("db down")).Once()
	_, svcErr = s.service.ProjectHasAuthorizationDetailTypes(s.ctx, "project-3")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}
