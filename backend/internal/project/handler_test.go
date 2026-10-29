// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ProjectHandlerTestSuite struct {
	suite.Suite
	mockService *ProjectServiceInterfaceMock
	handler     *projectHandler
}

func TestProjectHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectHandlerTestSuite))
}

func (suite *ProjectHandlerTestSuite) SetupTest() {
	suite.mockService = NewProjectServiceInterfaceMock(suite.T())
	suite.handler = newProjectHandler(suite.mockService)
}

func (suite *ProjectHandlerTestSuite) errorCode(rec *httptest.ResponseRecorder) string {
	var body apierror.ErrorResponse
	suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Code
}

func (suite *ProjectHandlerTestSuite) TestHandleProjectPostRequest() {
	suite.Run("creates project", func() {
		suite.SetupTest()
		suite.mockService.On("CreateProject", mock.Anything, ProjectRequest{
			Handle: "shop", DisplayName: "Shop",
		}).Return(&Project{ID: testProjectID, Handle: "shop", DisplayName: "Shop", RootOUID: testRootOUID}, nil)
		req := httptest.NewRequest(http.MethodPost, projectsPath,
			strings.NewReader(`{"handle":"shop","displayName":"Shop"}`))
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectPostRequest(rec, req)

		suite.Equal(http.StatusCreated, rec.Code)
		var body Project
		suite.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
		suite.Equal(testRootOUID, body.RootOUID)
	})
	suite.Run("malformed body", func() {
		suite.SetupTest()
		req := httptest.NewRequest(http.MethodPost, projectsPath, strings.NewReader(`{`))
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectPostRequest(rec, req)

		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ErrorInvalidRequestFormat.Code, suite.errorCode(rec))
	})
	suite.Run("handle conflict", func() {
		suite.SetupTest()
		suite.mockService.On("CreateProject", mock.Anything, mock.Anything).
			Return(nil, &ou.ErrorOrganizationUnitHandleConflict)
		req := httptest.NewRequest(http.MethodPost, projectsPath,
			strings.NewReader(`{"handle":"shop","displayName":"Shop"}`))
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectPostRequest(rec, req)

		suite.Equal(http.StatusConflict, rec.Code)
	})
}

func (suite *ProjectHandlerTestSuite) TestHandleProjectListRequest() {
	suite.Run("uses default page size", func() {
		suite.SetupTest()
		suite.mockService.On("GetProjectList", mock.Anything, 30, 0).
			Return(&ProjectListResponse{TotalResults: 0, StartIndex: 1, Projects: []Project{}}, nil)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectListRequest(rec, httptest.NewRequest(http.MethodGet, projectsPath, nil))

		suite.Equal(http.StatusOK, rec.Code)
	})
	suite.Run("passes limit and offset", func() {
		suite.SetupTest()
		suite.mockService.On("GetProjectList", mock.Anything, 5, 10).
			Return(&ProjectListResponse{StartIndex: 11, Projects: []Project{}}, nil)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectListRequest(rec,
			httptest.NewRequest(http.MethodGet, projectsPath+"?limit=5&offset=10", nil))

		suite.Equal(http.StatusOK, rec.Code)
	})
	suite.Run("non numeric limit", func() {
		suite.SetupTest()
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectListRequest(rec, httptest.NewRequest(http.MethodGet, projectsPath+"?limit=x", nil))

		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ErrorInvalidLimit.Code, suite.errorCode(rec))
	})
	suite.Run("non numeric offset", func() {
		suite.SetupTest()
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectListRequest(rec, httptest.NewRequest(http.MethodGet, projectsPath+"?offset=x", nil))

		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ErrorInvalidOffset.Code, suite.errorCode(rec))
	})
	suite.Run("service error", func() {
		suite.SetupTest()
		suite.mockService.On("GetProjectList", mock.Anything, 30, 0).Return(nil, &tidcommon.InternalServerError)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectListRequest(rec, httptest.NewRequest(http.MethodGet, projectsPath, nil))

		suite.Equal(http.StatusInternalServerError, rec.Code)
	})
}

func (suite *ProjectHandlerTestSuite) TestHandleProjectGetRequest() {
	suite.Run("found", func() {
		suite.SetupTest()
		suite.mockService.On("GetProject", mock.Anything, testProjectID).Return(&Project{ID: testProjectID}, nil)
		req := httptest.NewRequest(http.MethodGet, projectsPath+"/"+testProjectID, nil)
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectGetRequest(rec, req)

		suite.Equal(http.StatusOK, rec.Code)
	})
	suite.Run("not found", func() {
		suite.SetupTest()
		suite.mockService.On("GetProject", mock.Anything, testProjectID).Return(nil, &ErrorProjectNotFound)
		req := httptest.NewRequest(http.MethodGet, projectsPath+"/"+testProjectID, nil)
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectGetRequest(rec, req)

		suite.Equal(http.StatusNotFound, rec.Code)
	})
}

func (suite *ProjectHandlerTestSuite) TestHandleProjectPutRequest() {
	suite.Run("updates project", func() {
		suite.SetupTest()
		suite.mockService.On("UpdateProject", mock.Anything, testProjectID, ProjectRequest{DisplayName: "Store"}).
			Return(&Project{ID: testProjectID, DisplayName: "Store"}, nil)
		req := httptest.NewRequest(http.MethodPut, projectsPath+"/"+testProjectID,
			strings.NewReader(`{"displayName":"Store"}`))
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectPutRequest(rec, req)

		suite.Equal(http.StatusOK, rec.Code)
	})
	suite.Run("malformed body", func() {
		suite.SetupTest()
		req := httptest.NewRequest(http.MethodPut, projectsPath+"/"+testProjectID, strings.NewReader(`[`))
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectPutRequest(rec, req)

		suite.Equal(http.StatusBadRequest, rec.Code)
	})
	suite.Run("service error", func() {
		suite.SetupTest()
		suite.mockService.On("UpdateProject", mock.Anything, testProjectID, mock.Anything).
			Return(nil, &ErrorHandleChangeNotAllowed)
		req := httptest.NewRequest(http.MethodPut, projectsPath+"/"+testProjectID,
			strings.NewReader(`{"handle":"other","displayName":"Store"}`))
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectPutRequest(rec, req)

		suite.Equal(http.StatusBadRequest, rec.Code)
	})
}

func (suite *ProjectHandlerTestSuite) TestHandleProjectDeleteRequest() {
	suite.Run("deletes project", func() {
		suite.SetupTest()
		suite.mockService.On("DeleteProject", mock.Anything, testProjectID).Return(nil)
		req := httptest.NewRequest(http.MethodDelete, projectsPath+"/"+testProjectID, nil)
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectDeleteRequest(rec, req)

		suite.Equal(http.StatusNoContent, rec.Code)
	})
	suite.Run("root OU in use", func() {
		suite.SetupTest()
		suite.mockService.On("DeleteProject", mock.Anything, testProjectID).
			Return(&ou.ErrorCannotDeleteOrganizationUnit)
		req := httptest.NewRequest(http.MethodDelete, projectsPath+"/"+testProjectID, nil)
		req.SetPathValue("id", testProjectID)
		rec := httptest.NewRecorder()

		suite.handler.HandleProjectDeleteRequest(rec, req)

		suite.Equal(http.StatusBadRequest, rec.Code)
		suite.Equal(ou.ErrorCannotDeleteOrganizationUnit.Code, suite.errorCode(rec))
	})
}

func (suite *ProjectHandlerTestSuite) TestWriteProjectErrorStatusMapping() {
	cases := []struct {
		err  tidcommon.ServiceError
		want int
	}{
		{ErrorProjectNotFound, http.StatusNotFound},
		{ErrorProjectIDConflict, http.StatusConflict},
		{ou.ErrorOrganizationUnitHandleConflict, http.StatusConflict},
		{ou.ErrorOrganizationUnitNameConflict, http.StatusConflict},
		{tidcommon.ErrorUnauthorized, http.StatusForbidden},
		{ErrorInvalidDisplayName, http.StatusBadRequest},
		{tidcommon.InternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		suite.Run(tc.err.Code, func() {
			rec := httptest.NewRecorder()
			writeProjectError(context.Background(), rec, &tc.err)
			suite.Equal(tc.want, rec.Code)
		})
	}
}
