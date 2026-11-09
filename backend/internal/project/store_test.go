// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type ProjectStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *projectStore
}

func TestProjectStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectStoreTestSuite))
}

func (suite *ProjectStoreTestSuite) SetupTest() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	}))
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &projectStore{dbProvider: suite.mockDBProvider}
}

func (suite *ProjectStoreTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *ProjectStoreTestSuite) projectRow() map[string]interface{} {
	return map[string]interface{}{
		"id": testProjectID, "handle": []byte("shop"), "display_name": "Shop", "description": testDescription,
		"created_at": "2026-10-23 10:15:00",
	}
}

// expectedProject is the project projectRow describes.
func (suite *ProjectStoreTestSuite) expectedProject() Project {
	return Project{
		ID: testProjectID, Handle: "shop", DisplayName: "Shop", Description: testDescription, CreatedAt: testCreatedAt,
	}
}

func (suite *ProjectStoreTestSuite) withClient() {
	suite.mockDBProvider.On("GetEntityDBClient").Return(suite.mockDBClient, nil)
}

func (suite *ProjectStoreTestSuite) TestCreateProject() {
	suite.withClient()
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateProject, testProjectID, "shop", "Shop",
		testDescription, testCreatedAt, testDeploymentID).Return(int64(1), nil)

	suite.NoError(suite.store.CreateProject(context.Background(), suite.expectedProject()))
}

func (suite *ProjectStoreTestSuite) TestCreateProject_UniqueViolations() {
	cases := map[string]struct {
		dbErr   error
		wantErr error
	}{
		"postgres id":     {&pq.Error{Code: "23505", Constraint: primaryKeyName}, errProjectIDConflict},
		"postgres handle": {&pq.Error{Code: "23505", Constraint: handleIndexName}, errProjectHandleConflict},
		"sqlite id": {errors.New("constraint failed: UNIQUE constraint failed: " + primaryKeyColumns + " (1555)"),
			errProjectIDConflict},
		"sqlite handle": {errors.New("constraint failed: UNIQUE constraint failed: " + handleIndexColumns + " (2067)"),
			errProjectHandleConflict},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.withClient()
			suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateProject, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), tc.dbErr)

			suite.ErrorIs(suite.store.CreateProject(context.Background(), suite.expectedProject()), tc.wantErr)
		})
	}
	suite.Run("other error", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateProject, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("db"))

		err := suite.store.CreateProject(context.Background(), suite.expectedProject())

		suite.Error(err)
		suite.NotErrorIs(err, errProjectIDConflict)
		suite.NotErrorIs(err, errProjectHandleConflict)
	})
}

func (suite *ProjectStoreTestSuite) TestGetProject() {
	suite.Run("found", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectByID, testProjectID, testDeploymentID).
			Return([]map[string]interface{}{suite.projectRow()}, nil)

		project, err := suite.store.GetProject(context.Background(), testProjectID)

		suite.NoError(err)
		suite.Equal(suite.expectedProject(), project)
	})
	suite.Run("not found", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectByID, testProjectID, testDeploymentID).
			Return([]map[string]interface{}{}, nil)

		_, err := suite.store.GetProject(context.Background(), testProjectID)

		suite.ErrorIs(err, errProjectNotFound)
	})
	suite.Run("query error", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectByID, testProjectID, testDeploymentID).
			Return(nil, errors.New("db"))

		_, err := suite.store.GetProject(context.Background(), testProjectID)

		suite.Error(err)
		suite.NotErrorIs(err, errProjectNotFound)
	})
}

func (suite *ProjectStoreTestSuite) TestGetProjectByHandle() {
	suite.withClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectByHandle, "shop", testDeploymentID).
		Return([]map[string]interface{}{suite.projectRow()}, nil)

	project, err := suite.store.GetProjectByHandle(context.Background(), "shop")

	suite.NoError(err)
	suite.Equal(suite.expectedProject(), project)
}

func (suite *ProjectStoreTestSuite) TestUpdateProject() {
	suite.Run("updates the display name and description", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateProject, testProjectID, "Shop",
			testDescription, testDeploymentID).Return(int64(1), nil)

		suite.NoError(suite.store.UpdateProject(context.Background(), suite.expectedProject()))
	})
	suite.Run("execute error", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateProject, testProjectID, "Shop",
			testDescription, testDeploymentID).Return(int64(0), errors.New("db"))

		suite.Error(suite.store.UpdateProject(context.Background(), suite.expectedProject()))
	})
	suite.Run("client error", func() {
		suite.SetupTest()
		suite.mockDBProvider.On("GetEntityDBClient").Return(nil, errors.New("no client"))

		suite.Error(suite.store.UpdateProject(context.Background(), suite.expectedProject()))
	})
}

func (suite *ProjectStoreTestSuite) TestGetProjectList() {
	suite.withClient()
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectList, 10, 0, testDeploymentID).
		Return([]map[string]interface{}{suite.projectRow()}, nil)

	projects, err := suite.store.GetProjectList(context.Background(), 10, 0)

	suite.NoError(err)
	suite.Equal([]Project{suite.expectedProject()}, projects)
}

func (suite *ProjectStoreTestSuite) TestGetProjectListCount() {
	suite.Run("returns total", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectListCount, testDeploymentID).
			Return([]map[string]interface{}{{"total": int64(4)}}, nil)

		total, err := suite.store.GetProjectListCount(context.Background())

		suite.NoError(err)
		suite.Equal(4, total)
	})
	suite.Run("unexpected column type", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectListCount, testDeploymentID).
			Return([]map[string]interface{}{{"total": "4"}}, nil)

		_, err := suite.store.GetProjectListCount(context.Background())

		suite.Error(err)
	})
}

func (suite *ProjectStoreTestSuite) TestDeleteProject() {
	suite.Run("deletes", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteProject, testProjectID, testDeploymentID).
			Return(int64(1), nil)

		suite.NoError(suite.store.DeleteProject(context.Background(), testProjectID))
	})
	suite.Run("execute error", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteProject, testProjectID, testDeploymentID).
			Return(int64(0), errors.New("db"))

		suite.Error(suite.store.DeleteProject(context.Background(), testProjectID))
	})
	suite.Run("no project has the ID", func() {
		suite.SetupTest()
		suite.withClient()
		suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteProject, testProjectID, testDeploymentID).
			Return(int64(0), nil)

		suite.ErrorIs(suite.store.DeleteProject(context.Background(), testProjectID), errProjectNotFound)
	})
}

func (suite *ProjectStoreTestSuite) TestUnreadableCreatedAt() {
	suite.withClient()
	row := suite.projectRow()
	row["created_at"] = 42
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetProjectList, 10, 0, testDeploymentID).
		Return([]map[string]interface{}{row}, nil)

	_, err := suite.store.GetProjectList(context.Background(), 10, 0)

	suite.Error(err)
}

func (suite *ProjectStoreTestSuite) TestDBClientUnavailable() {
	suite.mockDBProvider.On("GetEntityDBClient").Return(nil, errors.New("no client"))
	ctx := context.Background()

	suite.Error(suite.store.CreateProject(ctx, Project{}))
	_, err := suite.store.GetProject(ctx, testProjectID)
	suite.Error(err)
	_, err = suite.store.GetProjectList(ctx, 10, 0)
	suite.Error(err)
	_, err = suite.store.GetProjectListCount(ctx)
	suite.Error(err)
	suite.Error(suite.store.DeleteProject(ctx, testProjectID))
}
