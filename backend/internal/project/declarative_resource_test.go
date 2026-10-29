// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ProjectExporterTestSuite struct {
	suite.Suite
	mockService *ProjectServiceInterfaceMock
	exporter    *projectExporter
}

func TestProjectExporterTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectExporterTestSuite))
}

func (suite *ProjectExporterTestSuite) SetupTest() {
	suite.mockService = NewProjectServiceInterfaceMock(suite.T())
	suite.exporter = newProjectExporter(suite.mockService)
}

func (suite *ProjectExporterTestSuite) TestTypes() {
	suite.Equal("project", suite.exporter.GetResourceType())
	suite.Equal("Project", suite.exporter.GetParameterizerType())
	suite.Empty(suite.exporter.GetResourceRules().Variables)
}

func (suite *ProjectExporterTestSuite) TestGetAllResourceIDs() {
	suite.Run("reads every page", func() {
		suite.SetupTest()
		suite.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(&ProjectListResponse{TotalResults: 2, Projects: []Project{{ID: "p1"}}}, nil)
		suite.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 1).
			Return(&ProjectListResponse{TotalResults: 2, Projects: []Project{{ID: "p2"}}}, nil)

		ids, svcErr := suite.exporter.GetAllResourceIDs(context.Background())

		suite.Nil(svcErr)
		suite.Equal([]string{"p1", "p2"}, ids)
	})
	suite.Run("no projects", func() {
		suite.SetupTest()
		suite.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(&ProjectListResponse{Projects: []Project{}}, nil)

		ids, svcErr := suite.exporter.GetAllResourceIDs(context.Background())

		suite.Nil(svcErr)
		suite.Empty(ids)
	})
	suite.Run("service error", func() {
		suite.SetupTest()
		suite.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(nil, &tidcommon.InternalServerError)

		_, svcErr := suite.exporter.GetAllResourceIDs(context.Background())

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
}

func (suite *ProjectExporterTestSuite) TestGetResourceByID() {
	suite.Run("exports the project with its root organization unit ID", func() {
		suite.SetupTest()
		suite.mockService.On("GetProject", mock.Anything, testProjectID).Return(&Project{
			ID: testProjectID, Handle: "shop", DisplayName: "Shop", Description: "Online shop",
			RootOUID: testRootOUID, CreatedAt: testCreatedAt,
		}, nil)

		resource, name, svcErr := suite.exporter.GetResourceByID(context.Background(), testProjectID)

		suite.Nil(svcErr)
		suite.Equal("Shop", name)
		suite.Equal(&projectDocument{
			ID: testProjectID, RootOUID: testRootOUID, Handle: "shop", DisplayName: "Shop",
			Description: "Online shop",
		}, resource)
	})
	suite.Run("service error", func() {
		suite.SetupTest()
		suite.mockService.On("GetProject", mock.Anything, testProjectID).Return(nil, &ErrorProjectNotFound)

		_, _, svcErr := suite.exporter.GetResourceByID(context.Background(), testProjectID)

		suite.Equal(ErrorProjectNotFound, *svcErr)
	})
}

func (suite *ProjectExporterTestSuite) TestValidateResource() {
	logger := log.GetLogger()

	name, exportErr := suite.exporter.ValidateResource(context.Background(),
		&projectDocument{ID: testProjectID, DisplayName: "Shop"}, testProjectID, logger)
	suite.Nil(exportErr)
	suite.Equal("Shop", name)

	_, exportErr = suite.exporter.ValidateResource(context.Background(), "not a project", testProjectID, logger)
	suite.NotNil(exportErr)

	_, exportErr = suite.exporter.ValidateResource(context.Background(),
		&projectDocument{ID: testProjectID}, testProjectID, logger)
	suite.NotNil(exportErr)
}
