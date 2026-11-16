// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/outype"
)

const testOUTypeID = "01900000-0000-7000-8000-000000000012"

var testOUTypeDocument = strings.Join([]string{
	"resource_type: ou_type",
	"id: " + testOUTypeID,
	"handle: partner",
	"displayName: Partner",
	"ouHandle: default",
	"systemAttributes:",
	"  display: registrationNo",
	"schema:",
	"  registrationNo:",
	"    type: string",
	"",
}, "\n")

type ImportOUTypeTestSuite struct {
	suite.Suite
	ouTypes *ouTypeAdapterMock
	service *importService
}

func TestImportOUTypeTestSuite(t *testing.T) {
	suite.Run(t, new(ImportOUTypeTestSuite))
}

func (s *ImportOUTypeTestSuite) SetupTest() {
	s.ouTypes = newOuTypeAdapterMock(s.T())
	s.service = newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, discardingBus{}).(*importService)
	s.service.ouTypeService = s.ouTypes
}

func (s *ImportOUTypeTestSuite) importDocument(content string, options *ImportOptions) ImportItemOutcome {
	s.T().Helper()
	resp, err := s.service.ImportResources(context.Background(), &ImportRequest{Content: content, Options: options})
	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	return resp.Results[0]
}

func (s *ImportOUTypeTestSuite) TestCreatesThroughTheOUTypeServiceKeepingTheID() {
	s.ouTypes.On("GetOUType", mock.Anything, testOUTypeID, false).Return(nil, &outype.ErrorOUTypeNotFound).Once()
	s.ouTypes.On("CreateOUType", mock.Anything, mock.MatchedBy(func(req outype.CreateOUTypeRequest) bool {
		return req.ID == testOUTypeID && req.Handle == "partner" && req.DisplayName == "Partner" &&
			req.OUHandle == "default" && req.SystemAttributes.Display == "registrationNo" &&
			string(req.Schema) == `{"registrationNo":{"type":"string"}}`
	})).Return(&outype.OUType{ID: testOUTypeID, DisplayName: "Partner"}, nil).Once()

	outcome := s.importDocument(testOUTypeDocument, nil)

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Equal(operationCreate, outcome.Operation)
	s.Equal(resourceTypeOUType, outcome.ResourceType)
	s.Equal(testOUTypeID, outcome.ResourceID)
}

func (s *ImportOUTypeTestSuite) TestUpsertUpdatesAnExistingType() {
	s.ouTypes.On("GetOUType", mock.Anything, testOUTypeID, false).
		Return(&outype.OUType{ID: testOUTypeID}, nil).Once()
	s.ouTypes.On("UpdateOUType", mock.Anything, testOUTypeID, mock.MatchedBy(func(req outype.UpdateOUTypeRequest) bool {
		return req.Handle == "partner" && req.OUHandle == "default"
	})).Return(&outype.OUType{ID: testOUTypeID, DisplayName: "Partner"}, nil).Once()

	outcome := s.importDocument(testOUTypeDocument, nil)

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Equal(operationUpdate, outcome.Operation)
}

func (s *ImportOUTypeTestSuite) TestCreateWithoutUpsertSkipsTheLookup() {
	upsert := false
	s.ouTypes.On("CreateOUType", mock.Anything, mock.Anything).
		Return(&outype.OUType{ID: testOUTypeID, DisplayName: "Partner"}, nil).Once()

	outcome := s.importDocument(testOUTypeDocument, &ImportOptions{Upsert: &upsert})

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Equal(operationCreate, outcome.Operation)
}

func (s *ImportOUTypeTestSuite) TestDryRunWritesNothing() {
	s.ouTypes.On("GetOUType", mock.Anything, testOUTypeID, false).Return(nil, &outype.ErrorOUTypeNotFound).Once()

	resp, err := s.service.ImportResources(context.Background(), &ImportRequest{
		Content: testOUTypeDocument, DryRun: true,
	})

	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	s.Equal(statusSuccess, resp.Results[0].Status, resp.Results[0].Message)
	s.Equal(operationCreate, resp.Results[0].Operation)
}

func (s *ImportOUTypeTestSuite) TestReportsTheOUTypeServiceRefusal() {
	s.ouTypes.On("GetOUType", mock.Anything, testOUTypeID, false).Return(nil, &outype.ErrorOUTypeNotFound).Once()
	s.ouTypes.On("CreateOUType", mock.Anything, mock.Anything).
		Return(nil, &outype.ErrorCredentialAttributeNotAllowed).Once()

	outcome := s.importDocument(testOUTypeDocument, nil)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(outype.ErrorCredentialAttributeNotAllowed.Code, outcome.Code)
}

func (s *ImportOUTypeTestSuite) TestFailsWhenTheLookupFails() {
	s.ouTypes.On("GetOUType", mock.Anything, testOUTypeID, false).
		Return(nil, &tidcommon.InternalServerError).Once()

	outcome := s.importDocument(testOUTypeDocument, nil)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(tidcommon.InternalServerError.Code, outcome.Code)
}

func (s *ImportOUTypeTestSuite) TestUnsupportedWithoutTheOUTypeService() {
	s.service.ouTypeService = nil

	outcome := s.importDocument(testOUTypeDocument, nil)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(resourceTypeOUType, outcome.ResourceType)
}

func (s *ImportOUTypeTestSuite) TestDeletesThroughTheOUTypeService() {
	s.ouTypes.On("DeleteOUType", mock.Anything, testOUTypeID).Return(nil).Once()

	deleteFn := s.service.deleterFor(resourceTypeOUType)

	s.Require().NotNil(deleteFn)
	s.Nil(deleteFn(context.Background(), testOUTypeID))
}

func (s *ImportOUTypeTestSuite) TestTypeSchemaJSON() {
	fromString, err := typeSchemaJSON(`{"region":{"type":"string"}}`)
	s.Require().NoError(err)
	s.JSONEq(`{"region":{"type":"string"}}`, string(fromString))

	fromYAML, err := typeSchemaJSON(map[string]interface{}{"region": map[string]interface{}{"type": "string"}})
	s.Require().NoError(err)
	s.JSONEq(`{"region":{"type":"string"}}`, string(fromYAML))

	_, err = typeSchemaJSON(map[string]interface{}{"bad": func() {}})
	s.Require().Error(err)
	var unsupportedErr *json.UnsupportedTypeError
	s.ErrorAs(err, &unsupportedErr)
}
