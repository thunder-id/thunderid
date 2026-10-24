// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// recordingCapturer records every resource handed to it.
type recordingCapturer struct {
	resourceTypes []string
	resources     []interface{}
}

func (r *recordingCapturer) CaptureValues(_ context.Context, resourceType string, resource interface{}) {
	r.resourceTypes = append(r.resourceTypes, resourceType)
	r.resources = append(r.resources, resource)
}

// recordCaptures hands the suite's service a capturer and returns it.
func (suite *ResourceServiceTestSuite) recordCaptures() *recordingCapturer {
	capturer := &recordingCapturer{}
	suite.service.(*resourceService).valueCapturer = capturer
	return capturer
}

// captured returns the one resource server handed to the capturer.
func (suite *ResourceServiceTestSuite) captured(capturer *recordingCapturer) *providers.ResourceServer {
	suite.Require().Len(capturer.resources, 1)
	suite.Equal(resourceTypeResourceServer, capturer.resourceTypes[0])
	server, ok := capturer.resources[0].(*providers.ResourceServer)
	suite.Require().True(ok, "the capture was not handed the shape the exporter reads back")
	return server
}

func (suite *ResourceServiceTestSuite) expectCreate(rs providers.ResourceServer, storeErr error) {
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, rs.OUID).
		Return(oupkg.OrganizationUnit{ID: rs.OUID}, nil)
	suite.mockStore.On("CheckResourceServerNameExists", mock.Anything, rs.Name).Return(false, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything, rs.Identifier).Return(false, nil)
	suite.mockStore.On("CreateResourceServer", mock.Anything, mock.AnythingOfType("string"),
		matchResourceServer(rs)).Return(storeErr)
}

// A created resource server is captured as the create returns it, identifier and all.
func (suite *ResourceServiceTestSuite) TestCreateResourceServer_CapturesTheCreatedServer() {
	capturer := suite.recordCaptures()
	rs := providers.ResourceServer{Name: "Orders API", Identifier: "https://orders.example.com", OUID: "ou-123"}
	suite.expectCreate(rs, nil)

	result, svcErr := suite.service.CreateResourceServer(context.Background(), rs)
	suite.Require().Nil(svcErr)

	server := suite.captured(capturer)
	suite.Equal(result, server)
	suite.Equal("Orders API", server.Name)
	suite.Equal("https://orders.example.com", server.Identifier)
}

// A create that fails captures nothing.
func (suite *ResourceServiceTestSuite) TestCreateResourceServer_FailureCapturesNothing() {
	capturer := suite.recordCaptures()
	rs := providers.ResourceServer{Name: "Orders API", Identifier: "https://orders.example.com", OUID: "ou-123"}
	suite.expectCreate(rs, errors.New("database error"))

	_, svcErr := suite.service.CreateResourceServer(context.Background(), rs)

	suite.Require().NotNil(svcErr)
	suite.Empty(capturer.resources)
}

// An updated resource server is captured as the update returns it, so a changed identifier replaces
// the one the default gateway held.
func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_CapturesTheUpdatedServer() {
	capturer := suite.recordCaptures()
	existing := providers.ResourceServer{
		ID: "rs-123", Name: "Orders API", Identifier: "https://old.example.com", OUID: "ou-123", Delimiter: ":",
	}
	suite.mockStore.On("IsResourceServerDeclarative", "rs-123").Return(false)
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").Return(existing, nil)
	suite.mockStore.On("CheckResourceServerIdentifierExists", mock.Anything,
		"https://orders.example.com").Return(false, nil)
	suite.mockOU.On("GetOrganizationUnit", mock.Anything, "ou-123").
		Return(oupkg.OrganizationUnit{ID: "ou-123"}, nil)
	suite.mockStore.On("UpdateResourceServer", mock.Anything, "rs-123", mock.Anything).Return(nil)

	result, svcErr := suite.service.UpdateResourceServer(context.Background(), "rs-123", providers.ResourceServer{
		Name: "Orders API", Identifier: "https://orders.example.com", OUID: "ou-123",
	})
	suite.Require().Nil(svcErr)

	server := suite.captured(capturer)
	suite.Equal(result, server)
	suite.Equal("rs-123", server.ID)
	suite.Equal("https://orders.example.com", server.Identifier)
}

// An update that fails captures nothing.
func (suite *ResourceServiceTestSuite) TestUpdateResourceServer_FailureCapturesNothing() {
	capturer := suite.recordCaptures()
	suite.mockStore.On("GetResourceServer", mock.Anything, "rs-123").
		Return(providers.ResourceServer{}, errResourceServerNotFound)

	_, svcErr := suite.service.UpdateResourceServer(context.Background(), "rs-123", providers.ResourceServer{
		Name: "Orders API", Identifier: "https://orders.example.com", OUID: "ou-123",
	})

	suite.Require().NotNil(svcErr)
	suite.Empty(capturer.resources)
}

// Without a capturer, which is every plane but a control plane, nothing is captured.
func (suite *ResourceServiceTestSuite) TestCaptureWithoutACapturerDoesNothing() {
	service := suite.service.(*resourceService)
	suite.Nil(service.valueCapturer)

	suite.NotPanics(func() {
		service.captureValues(context.Background(), &providers.ResourceServer{Name: "Orders API"})
	})
}

// A capturer is not handed a server that is not there.
func (suite *ResourceServiceTestSuite) TestCaptureOfNoServerDoesNothing() {
	capturer := suite.recordCaptures()

	suite.service.(*resourceService).captureValues(context.Background(), nil)

	suite.Empty(capturer.resources)
}
