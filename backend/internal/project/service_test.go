// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

const (
	testProjectID   = "019a0000-0000-7000-8000-000000000001"
	testHandle      = "shop"
	testDisplayName = "Shop"
	testDescription = "Online shop"
)

var testCreatedAt = time.Date(2026, 10, 23, 10, 15, 0, 0, time.UTC)

type ProjectServiceTestSuite struct {
	suite.Suite
	mockStore     *projectStoreInterfaceMock
	mockOUService *oumock.OrganizationUnitServiceInterfaceMock
	service       *projectService
}

func TestProjectServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectServiceTestSuite))
}

func (suite *ProjectServiceTestSuite) SetupTest() {
	suite.mockStore = newProjectStoreInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	mockTx := transactionmock.NewTransactionerMock(suite.T())
	mockTx.On("Transact", mock.Anything, mock.Anything).
		Return(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
	suite.service = newProjectService(suite.mockStore, mockTx, suite.mockOUService)
}

// project is the project the store holds in these tests.
func (suite *ProjectServiceTestSuite) project() Project {
	return Project{
		ID: testProjectID, Handle: "shop", DisplayName: "Shop", Description: testDescription,
		CreatedAt: testCreatedAt,
	}
}

func (suite *ProjectServiceTestSuite) TestCreateProject_StoresProject() {
	suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errProjectNotFound)
	suite.mockStore.On("CreateProject", mock.Anything, mock.MatchedBy(func(r Project) bool {
		return r.ID != "" && r.Handle == testHandle && r.DisplayName == testDisplayName &&
			r.Description == testDescription && !r.CreatedAt.IsZero()
	})).Return(nil)

	created, svcErr := suite.service.CreateProject(context.Background(),
		ProjectRequest{Handle: "shop", DisplayName: "Shop", Description: testDescription})

	suite.Nil(svcErr)
	suite.NotEmpty(created.ID)
	suite.Equal("shop", created.Handle)
	suite.Equal("Shop", created.DisplayName)
	suite.Equal(testDescription, created.Description)
	suite.mockOUService.AssertNotCalled(suite.T(), "CreateOrganizationUnit", mock.Anything, mock.Anything)
}

func (suite *ProjectServiceTestSuite) TestCreateProject_KeepsSuppliedID() {
	suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
	suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errProjectNotFound)
	suite.mockStore.On("CreateProject", mock.Anything, mock.MatchedBy(func(r Project) bool {
		return r.ID == testProjectID
	})).Return(nil)

	created, svcErr := suite.service.CreateProject(context.Background(),
		ProjectRequest{ID: testProjectID, Handle: "shop", DisplayName: "Shop", Description: testDescription})

	suite.Nil(svcErr)
	suite.Equal(testProjectID, created.ID)
	suite.False(created.CreatedAt.IsZero())
}

func (suite *ProjectServiceTestSuite) TestCreateProject_ValidationErrors() {
	cases := []struct {
		name    string
		request ProjectRequest
		wantErr tidcommon.ServiceError
	}{
		{"non uuid id", ProjectRequest{ID: "not-a-uuid", Handle: "shop", DisplayName: "Shop"}, ErrorInvalidProjectID},
		{"missing handle", ProjectRequest{DisplayName: "Shop"}, ErrorInvalidHandle},
		{"malformed handle", ProjectRequest{Handle: "Shop/1", DisplayName: "Shop"}, ErrorInvalidHandle},
		{"long handle", ProjectRequest{Handle: strings.Repeat("a", 101), DisplayName: "Shop"}, ErrorInvalidHandle},
		{"blank display name", ProjectRequest{Handle: "shop", DisplayName: "  "}, ErrorInvalidDisplayName},
		{"long display name", ProjectRequest{Handle: "shop", DisplayName: strings.Repeat("a", 101)},
			ErrorInvalidDisplayName},
		{"long description", ProjectRequest{Handle: "shop", DisplayName: "Shop",
			Description: strings.Repeat("a", 256)}, ErrorInvalidDescription},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			created, svcErr := suite.service.CreateProject(context.Background(), tc.request)

			suite.Nil(created)
			suite.Equal(tc.wantErr, *svcErr)
		})
	}
}

func (suite *ProjectServiceTestSuite) TestCreateProject_IDConflict() {
	suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)

	_, svcErr := suite.service.CreateProject(context.Background(),
		ProjectRequest{ID: testProjectID, Handle: "shop", DisplayName: "Shop"})

	suite.Equal(ErrorProjectIDConflict, *svcErr)
}

func (suite *ProjectServiceTestSuite) TestCreateProject_HandleConflict() {
	suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(suite.project(), nil)

	_, svcErr := suite.service.CreateProject(context.Background(), ProjectRequest{Handle: "shop", DisplayName: "Shop"})

	suite.Equal(ErrorHandleConflict, *svcErr)
	suite.mockStore.AssertNotCalled(suite.T(), "CreateProject", mock.Anything, mock.Anything)
}

func (suite *ProjectServiceTestSuite) TestCreateProject_StoreErrors() {
	suite.Run("id lookup fails", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errors.New("db"))

		_, svcErr := suite.service.CreateProject(context.Background(),
			ProjectRequest{ID: testProjectID, Handle: "shop", DisplayName: "Shop"})

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
	suite.Run("handle lookup fails", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errors.New("db"))

		_, svcErr := suite.service.CreateProject(context.Background(),
			ProjectRequest{Handle: "shop", DisplayName: "Shop"})

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
	suite.Run("insert loses an ID race", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
		suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errProjectNotFound)
		suite.mockStore.On("CreateProject", mock.Anything, mock.Anything).Return(errProjectIDConflict)

		_, svcErr := suite.service.CreateProject(context.Background(),
			ProjectRequest{ID: testProjectID, Handle: "shop", DisplayName: "Shop"})

		suite.Equal(ErrorProjectIDConflict, *svcErr)
	})
	suite.Run("insert loses a handle race", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errProjectNotFound)
		suite.mockStore.On("CreateProject", mock.Anything, mock.Anything).Return(errProjectHandleConflict)

		_, svcErr := suite.service.CreateProject(context.Background(),
			ProjectRequest{Handle: "shop", DisplayName: "Shop"})

		suite.Equal(ErrorHandleConflict, *svcErr)
	})
	suite.Run("insert fails", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errProjectNotFound)
		suite.mockStore.On("CreateProject", mock.Anything, mock.Anything).Return(errors.New("db"))

		_, svcErr := suite.service.CreateProject(context.Background(),
			ProjectRequest{Handle: "shop", DisplayName: "Shop"})

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
}

func (suite *ProjectServiceTestSuite) TestGetProject() {
	suite.Run("found", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)

		project, svcErr := suite.service.GetProject(context.Background(), testProjectID)

		suite.Nil(svcErr)
		suite.Equal(new(suite.project()), project)
	})
	suite.Run("not found", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)

		_, svcErr := suite.service.GetProject(context.Background(), testProjectID)

		suite.Equal(ErrorProjectNotFound, *svcErr)
	})
	suite.Run("store error", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errors.New("db"))

		_, svcErr := suite.service.GetProject(context.Background(), testProjectID)

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
}

func (suite *ProjectServiceTestSuite) TestResolveProjectOU() {
	for name, tc := range map[string]struct {
		ouID     string
		expected string
	}{
		"no organization unit":                   {expected: ou.SystemOUID},
		"an organization unit below the default": {ouID: "child-ou", expected: "child-ou"},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)
			if tc.ouID != "" {
				suite.mockOUService.On("IsParent", mock.Anything, ou.SystemOUID, tc.ouID).Return(true, nil)
			}

			ouID, svcErr := suite.service.ResolveProjectOU(context.Background(), testProjectID, tc.ouID)

			suite.Nil(svcErr)
			suite.Equal(tc.expected, ouID)
		})
	}
}

func (suite *ProjectServiceTestSuite) TestResolveProjectOU_Fails() {
	unknownOU := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "OU-1003"}
	for name, tc := range map[string]struct {
		storeErr error
		ouErr    *tidcommon.ServiceError
		expected tidcommon.ServiceError
	}{
		"an unknown project": {storeErr: errProjectNotFound, expected: ErrorProjectNotFound},
		"a store error": {
			storeErr: errors.New("db"), expected: tidcommon.InternalServerError,
		},
		"an organization unit outside the default": {expected: ErrorOUNotInProject},
		"an unknown organization unit":             {ouErr: unknownOU, expected: *unknownOU},
		"a hierarchy lookup failure": {
			ouErr: &tidcommon.InternalServerError, expected: tidcommon.InternalServerError,
		},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			if tc.storeErr != nil {
				suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, tc.storeErr)
			} else {
				suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)
				suite.mockOUService.On("IsParent", mock.Anything, ou.SystemOUID, "other-ou").Return(false, tc.ouErr)
			}

			ouID, svcErr := suite.service.ResolveProjectOU(context.Background(), testProjectID, "other-ou")

			suite.Empty(ouID)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.expected, *svcErr)
		})
	}
}

func (suite *ProjectServiceTestSuite) TestGetProjectList() {
	suite.Run("returns page", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectListCount", mock.Anything).Return(3, nil)
		suite.mockStore.On("GetProjectList", mock.Anything, 2, 0).Return([]Project{suite.project()}, nil)

		list, svcErr := suite.service.GetProjectList(context.Background(), 2, 0)

		suite.Nil(svcErr)
		suite.Equal(3, list.TotalResults)
		suite.Equal(1, list.StartIndex)
		suite.Equal([]Project{suite.project()}, list.Projects)
		suite.NotEmpty(list.Links)
	})
	suite.Run("invalid limit", func() {
		suite.SetupTest()
		_, svcErr := suite.service.GetProjectList(context.Background(), 0, 0)
		suite.Equal(ErrorInvalidLimit, *svcErr)
	})
	suite.Run("invalid offset", func() {
		suite.SetupTest()
		_, svcErr := suite.service.GetProjectList(context.Background(), 10, -1)
		suite.Equal(ErrorInvalidOffset, *svcErr)
	})
	suite.Run("count fails", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectListCount", mock.Anything).Return(0, errors.New("db"))
		_, svcErr := suite.service.GetProjectList(context.Background(), 10, 0)
		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
	suite.Run("list fails", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectListCount", mock.Anything).Return(1, nil)
		suite.mockStore.On("GetProjectList", mock.Anything, 10, 0).Return(nil, errors.New("db"))
		_, svcErr := suite.service.GetProjectList(context.Background(), 10, 0)
		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
}

func (suite *ProjectServiceTestSuite) TestUpdateProject() {
	suite.Run("updates the display name and description", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)
		suite.mockStore.On("UpdateProject", mock.Anything, mock.MatchedBy(func(r Project) bool {
			return r.ID == testProjectID && r.Handle == testHandle && r.DisplayName == "Store" &&
				r.Description == "Renamed"
		})).Return(nil)

		updated, svcErr := suite.service.UpdateProject(context.Background(), testProjectID,
			ProjectRequest{Handle: "shop", DisplayName: "Store", Description: "Renamed"})

		suite.Nil(svcErr)
		suite.Equal("Store", updated.DisplayName)
		suite.Equal("Renamed", updated.Description)
		suite.Equal("shop", updated.Handle)
	})
	suite.Run("handle change refused", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)

		_, svcErr := suite.service.UpdateProject(context.Background(), testProjectID,
			ProjectRequest{Handle: "other", DisplayName: "Shop"})

		suite.Equal(ErrorHandleChangeNotAllowed, *svcErr)
	})
	suite.Run("invalid display name", func() {
		suite.SetupTest()
		_, svcErr := suite.service.UpdateProject(context.Background(), testProjectID, ProjectRequest{})
		suite.Equal(ErrorInvalidDisplayName, *svcErr)
	})
	suite.Run("not found", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
		_, svcErr := suite.service.UpdateProject(context.Background(), testProjectID,
			ProjectRequest{DisplayName: "Shop"})
		suite.Equal(ErrorProjectNotFound, *svcErr)
	})
	suite.Run("store update fails", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)
		suite.mockStore.On("UpdateProject", mock.Anything, mock.Anything).Return(errors.New("db"))
		_, svcErr := suite.service.UpdateProject(context.Background(), testProjectID,
			ProjectRequest{DisplayName: "Store"})
		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
}

func (suite *ProjectServiceTestSuite) TestDeleteProject() {
	suite.Run("deletes the project and no organization unit", func() {
		suite.SetupTest()
		suite.mockStore.On("DeleteProject", mock.Anything, testProjectID).Return(nil)

		suite.Nil(suite.service.DeleteProject(context.Background(), testProjectID))
		suite.mockOUService.AssertNotCalled(suite.T(), "DeleteOrganizationUnit", mock.Anything, mock.Anything)
	})
	suite.Run("delete fails", func() {
		suite.SetupTest()
		suite.mockStore.On("DeleteProject", mock.Anything, testProjectID).Return(errors.New("db"))

		suite.Equal(tidcommon.InternalServerError, *suite.service.DeleteProject(context.Background(), testProjectID))
	})
}

func (suite *ProjectServiceTestSuite) TestGetResourceDependencies() {
	suite.Run("other resource type", func() {
		suite.SetupTest()
		deps, err := suite.service.GetResourceDependencies(context.Background(),
			resourcedependency.ResourceTypeGroup, ou.SystemOUID)
		suite.NoError(err)
		suite.Empty(deps)
	})
	suite.Run("other organization unit", func() {
		suite.SetupTest()
		deps, err := suite.service.GetResourceDependencies(context.Background(),
			resourcedependency.ResourceTypeOU, "other-ou")
		suite.NoError(err)
		suite.Empty(deps)
	})
	suite.Run("a project blocks deletion of the system organization unit", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectList", mock.Anything, 1, 0).
			Return([]Project{suite.project()}, nil)

		deps, err := suite.service.GetResourceDependencies(context.Background(),
			resourcedependency.ResourceTypeOU, ou.SystemOUID)

		suite.NoError(err)
		suite.Equal([]resourcedependency.ResourceDependency{{
			ResourceType:     resourcedependency.ResourceTypeProject,
			ID:               testProjectID,
			DisplayName:      "Shop",
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		}}, deps)
	})
	suite.Run("no projects", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectList", mock.Anything, 1, 0).Return([]Project{}, nil)

		deps, err := suite.service.GetResourceDependencies(context.Background(),
			resourcedependency.ResourceTypeOU, ou.SystemOUID)

		suite.NoError(err)
		suite.Empty(deps)
	})
	suite.Run("store error", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProjectList", mock.Anything, 1, 0).
			Return(nil, errors.New("db"))

		_, err := suite.service.GetResourceDependencies(context.Background(),
			resourcedependency.ResourceTypeOU, ou.SystemOUID)

		suite.Error(err)
	})
}

func (suite *ProjectServiceTestSuite) TestProjectExists() {
	suite.Run("exists", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)

		exists, svcErr := suite.service.ProjectExists(context.Background(), testProjectID)

		suite.Nil(svcErr)
		suite.True(exists)
	})
	suite.Run("missing", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)

		exists, svcErr := suite.service.ProjectExists(context.Background(), testProjectID)

		suite.Nil(svcErr)
		suite.False(exists)
	})
	suite.Run("store failure", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errors.New("db"))

		_, svcErr := suite.service.ProjectExists(context.Background(), testProjectID)

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
}

// importProject calls ImportProject with the test project's ID and details.
func (suite *ProjectServiceTestSuite) importProject(handle string, upsert bool) (bool, *tidcommon.ServiceError) {
	return suite.service.ImportProject(context.Background(), testProjectID, handle, "Shop", testDescription, upsert)
}

func (suite *ProjectServiceTestSuite) TestImportProject_Validation() {
	_, svcErr := suite.service.ImportProject(context.Background(), "not-a-uuid", "shop", "Shop", "", true)
	suite.Equal(ErrorInvalidProjectID, *svcErr)

	suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
	_, svcErr = suite.service.ImportProject(context.Background(), testProjectID, "", "Shop", "", true)
	suite.Equal(ErrorInvalidHandle, *svcErr)

	_, svcErr = suite.service.ImportProject(context.Background(), testProjectID, "shop", " ", "", true)
	suite.Equal(ErrorInvalidDisplayName, *svcErr)
}

func (suite *ProjectServiceTestSuite) TestImportProject_CreatesProject() {
	suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
	suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").Return(Project{}, errProjectNotFound)
	suite.mockStore.On("CreateProject", mock.Anything, mock.MatchedBy(func(r Project) bool {
		return r.ID == testProjectID && r.Handle == testHandle && r.DisplayName == testDisplayName &&
			r.Description == testDescription
	})).Return(nil)

	created, svcErr := suite.importProject("shop", true)

	suite.Nil(svcErr)
	suite.True(created)
}

func (suite *ProjectServiceTestSuite) TestImportProject_ExistingProject() {
	suite.Run("without upsert the ID conflicts", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)

		created, svcErr := suite.importProject("shop", false)

		suite.False(created)
		suite.Equal(ErrorProjectIDConflict, *svcErr)
	})
	suite.Run("a different handle is refused", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(suite.project(), nil)

		_, svcErr := suite.importProject("other", true)

		suite.Equal(ErrorHandleChangeNotAllowed, *svcErr)
	})
	suite.Run("upsert updates the display name and description", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).
			Return(Project{ID: testProjectID, Handle: "shop", DisplayName: "Old"}, nil)
		suite.mockStore.On("UpdateProject", mock.Anything, mock.MatchedBy(func(r Project) bool {
			return r.DisplayName == testDisplayName && r.Description == testDescription
		})).Return(nil)

		created, svcErr := suite.importProject("shop", true)

		suite.Nil(svcErr)
		suite.False(created)
	})
}

func (suite *ProjectServiceTestSuite) TestImportProject_Failures() {
	suite.Run("project lookup failure", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errors.New("db"))

		_, svcErr := suite.importProject("shop", true)

		suite.Equal(tidcommon.InternalServerError, *svcErr)
	})
	suite.Run("handle used by another project", func() {
		suite.SetupTest()
		suite.mockStore.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
		suite.mockStore.On("GetProjectByHandle", mock.Anything, "shop").
			Return(Project{ID: "019a0000-0000-7000-8000-0000000000ff", Handle: "shop"}, nil)

		created, svcErr := suite.importProject("shop", true)

		suite.True(created)
		suite.Equal(ErrorHandleConflict, *svcErr)
	})
}
